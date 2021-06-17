package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
)

const owner string = `@_**Maren Beam (SP2'19)**`

// this is the "id" field from zulip, and is a permanent user ID that's not secret
// Pairing Bot's owner can add their ID here for testing. ctrl+f "ownerID" to see where it's used
const ownerID = 215391

const oddOneOutMessage string = "OK this is awkward.\nThere were an odd number of people in the match-set today, which means that one person couldn't get paired. Unfortunately, it was you -- I'm really sorry :(\nI promise it's not personal, it was very much random. Hopefully this doesn't happen again too soon. Enjoy your day! <3"
const matchedMessage = "Hi you two! You've been matched for pairing :)\n\nHave fun!"
const offboardedMessage = "Hi! You've been unsubscribed from Pairing Bot.\n\nThis happens at the end of every batch, when everyone is offboarded even if they're still in batch. If you'd like to re-subscribe, just send me a message that says `subscribe`.\n\nBe well! :)"

const botEmailAddress = "pairing-bot@recurse.zulipchat.com"
const zulipAPIURL = "https://recurse.zulipchat.com/api/v1/messages"

var maintenanceMode = false

// This is a struct that gets only what
// we need from the incoming JSON payload
type incomingJSON struct {
	Data    string `json:"data"`
	Token   string `json:"token"`
	Trigger string `json:"trigger"`
	Message struct {
		SenderID         int         `json:"sender_id"`
		DisplayRecipient interface{} `json:"display_recipient"`
		SenderEmail      string      `json:"sender_email"`
		SenderFullName   string      `json:"sender_full_name"`
	} `json:"message"`
}

// Zulip has to get JSON back from the bot,
// this does that. An empty message field stops
// zulip from throwing an error at the user that
// messaged the bot, but doesn't send a response
type botResponse struct {
	Message string `json:"content"`
}

type botNoResponse struct {
	Message bool `json:"response_not_required"`
}

func sanityCheck(ctx context.Context, client *firestore.Client, w http.ResponseWriter, r *http.Request) (incomingJSON, error) {
	var userReq incomingJSON
	// Look at the incoming webhook and slurp up the JSON
	// Error if the JSON from Zulip istelf is bad
	err := json.NewDecoder(r.Body).Decode(&userReq)
	if err != nil {
		http.NotFound(w, r)
		return userReq, err
	}

	// validate our zulip-bot token
	// this was manually put into the database before deployment
	doc, err := client.Collection("botauth").Doc("token").Get(ctx)
	if err != nil {
		log.Println("Something weird happened trying to read the auth token from the database")
		return userReq, err
	}
	token := doc.Data()
	if userReq.Token != token["value"] {
		http.NotFound(w, r)
		return userReq, errors.New("unauthorized interaction attempt")
	}
	return userReq, err
}

func handle(w http.ResponseWriter, r *http.Request) {
	responder := json.NewEncoder(w)
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, "pairing-bot-284823")
	if err != nil {
		log.Panic(err)
	}
	// sanity check the incoming request
	userReq, err := sanityCheck(ctx, client, w, r)
	if err != nil {
		log.Println(err)
		return
	}

	// for testing only
	// this responds with a maintenance message and quits if the request is coming from anyone other than the owner
	if maintenanceMode {
		if userReq.Message.SenderID != ownerID {
			err = responder.Encode(botResponse{`pairing bot is down for maintenance`})
			if err != nil {
				log.Println(err)
			}
			return
		}
	}

	if userReq.Trigger != "private_message" {
		err = responder.Encode(botResponse{"Hi! I'm Pairing Bot (she/her)!\n\nSend me a PM that says `subscribe` to get started :smiley:\n\n:pear::robot:\n:octopus::octopus:"})
		if err != nil {
			log.Println(err)
		}
		return
	}
	// if there aren't two 'recipients' (one sender and one receiver),
	// then don't respond. this stops pairing bot from responding in the group
	// chat she starts when she matches people
	if len(userReq.Message.DisplayRecipient.([]interface{})) != 2 {
		err = responder.Encode(botNoResponse{true})
		if err != nil {
			log.Println(err)
		}
		return
	}
	// you *should* be able to throw any freakin string at this thing and get back a valid command for dispatch()
	// if there are no commad arguments, cmdArgs will be nil
	cmd, cmdArgs, err := parseCmd(userReq.Data)
	if err != nil {
		log.Println(err)
	}
	// the tofu and potatoes right here y'all
	response, err := dispatch(ctx, client, cmd, cmdArgs, strconv.Itoa(userReq.Message.SenderID), userReq.Message.SenderEmail, userReq.Message.SenderFullName)
	if err != nil {
		log.Println(err)
	}
	err = responder.Encode(botResponse{response})
	if err != nil {
		log.Println(err)
	}
}

func nope(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}

// "match" makes matches for pairing, and messages those people to notify them of their match
// it runs once per day at 8am (it's triggered with app engine's cron service)
func match(w http.ResponseWriter, r *http.Request) {
	// Check that the request is originating from within app engine
	// https://cloud.google.com/appengine/docs/flexible/go/scheduling-jobs-with-cron-yaml#validating_cron_requests
	if r.Header.Get("X-Appengine-Cron") != "true" {
		http.NotFound(w, r)
		return
	}

	// setting up database connection
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, "pairing-bot-284823")
	if err != nil {
		log.Panic(err)
	}
	var recursersList []map[string]interface{}
	var skippersList []map[string]interface{}
	// this gets the time from system time, which is UTC
	// on app engine (and most other places). This works
	// fine for us in NYC, but might not if pairing bot
	// were ever running in another time zone
	today := strings.ToLower(time.Now().Weekday().String())

	// ok this is how we have to get all the recursers. it's weird.
	// this query returns an iterator, and then we have to use firestore
	// magic to iterate across the results of the query and store them
	// into our 'recursersList' variable which is a slice of map[string]interface{}
	iter := client.Collection("recursers").Where("isSkippingTomorrow", "==", false).Where("schedule."+today, "==", true).Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Panic(err)
		}
		recursersList = append(recursersList, doc.Data())
	}

	// get everyone who was set to skip today and set them back to isSkippingTomorrow = false
	iter = client.Collection("recursers").Where("isSkippingTomorrow", "==", true).Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Panic(err)
		}
		skippersList = append(skippersList, doc.Data())
	}
	for i := range skippersList {
		skippersList[i]["isSkippingTomorrow"] = false
		_, err = client.Collection("recursers").Doc(skippersList[i]["id"].(string)).Set(ctx, skippersList[i], firestore.MergeAll)
		if err != nil {
			log.Println(err)
		}
	}

	// shuffle our recursers. This will not error if the list is empty
	recursersList = shuffle(recursersList)

	// if for some reason there's no matches today, we're done
	if len(recursersList) == 0 {
		log.Println("No one was signed up to pair today -- so there were no matches")
		return
	}

	// message the peeps!
	doc, err := client.Collection("apiauth").Doc("key").Get(ctx)
	if err != nil {
		log.Panic(err)
	}
	apikey := doc.Data()
	botUsername := botEmailAddress
	botPassword := apikey["value"].(string)
	zulipClient := &http.Client{}

	// if there's an odd number today, message the last person in the list
	// and tell them they don't get a match today, then knock them off the list
	if len(recursersList)%2 != 0 {
		recurser := recursersList[len(recursersList)-1]
		recursersList = recursersList[:len(recursersList)-1]
		log.Println("Someone was the odd-one-out today")
		messageRequest := url.Values{}
		messageRequest.Add("type", "private")
		messageRequest.Add("to", recurser["email"].(string))
		messageRequest.Add("content", oddOneOutMessage)
		req, err := http.NewRequest("POST", zulipAPIURL, strings.NewReader(messageRequest.Encode()))
		if err != nil {
			log.Println(err)
		}
		req.SetBasicAuth(botUsername, botPassword)
		req.Header.Set("content-type", "application/x-www-form-urlencoded")
		resp, err := zulipClient.Do(req)
		if err != nil {
			log.Panic(err)
		}
		defer resp.Body.Close()
		respBodyText, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Println(err)
		}
		log.Println(string(respBodyText))
	}

	for i := 0; i < len(recursersList); i += 2 {
		messageRequest := url.Values{}
		messageRequest.Add("type", "private")
		messageRequest.Add("to", recursersList[i]["email"].(string)+", "+recursersList[i+1]["email"].(string))
		messageRequest.Add("content", matchedMessage)
		req, err := http.NewRequest("POST", zulipAPIURL, strings.NewReader(messageRequest.Encode()))
		if err != nil {
			log.Println(err)
		}
		req.SetBasicAuth(botUsername, botPassword)
		req.Header.Set("content-type", "application/x-www-form-urlencoded")
		resp, err := zulipClient.Do(req)
		if err != nil {
			log.Panic(err)
		}
		defer resp.Body.Close()
		respBodyText, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Println(err)
		}
		log.Println(string(respBodyText))
		log.Println(recursersList[i]["email"].(string), "was", "matched", "with", recursersList[i+1]["email"].(string))
	}
}

func endofbatch(w http.ResponseWriter, r *http.Request) {
	// Check that the request is originating from within app engine
	// https://cloud.google.com/appengine/docs/flexible/go/scheduling-jobs-with-cron-yaml#validating_cron_requests
	if r.Header.Get("X-Appengine-Cron") != "true" {
		http.NotFound(w, r)
		return
	}

	// setting up database connection
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, "pairing-bot-284823")
	if err != nil {
		log.Panic(err)
	}
	var recursersList []map[string]interface{}

	// getting all the recursers
	iter := client.Collection("recursers").Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Panic(err)
		}
		recursersList = append(recursersList, doc.Data())
	}

	// message and offboard everyone (delete them from the database)
	doc, err := client.Collection("apiauth").Doc("key").Get(ctx)
	if err != nil {
		log.Panic(err)
	}
	apikey := doc.Data()
	botUsername := botEmailAddress
	botPassword := apikey["value"].(string)
	zulipClient := &http.Client{}

	for i := 0; i < len(recursersList); i++ {
		recurserID := recursersList[i]["id"].(string)
		recurserEmail := recursersList[i]["email"].(string)
		messageRequest := url.Values{}
		var message string

		_, err = client.Collection("recursers").Doc(recurserID).Delete(ctx)
		if err != nil {
			log.Println(err)
			message = fmt.Sprintf("Uh oh, I was trying to offboard you since it's the end of batch, but something went wrong. Consider messaging %v to let them know this happened.", owner)
		} else {
			log.Println("A user was offboarded because it's the end of a batch.")
			message = offboardedMessage
		}

		messageRequest.Add("type", "private")
		messageRequest.Add("to", recurserEmail)
		messageRequest.Add("content", message)
		req, err := http.NewRequest("POST", zulipAPIURL, strings.NewReader(messageRequest.Encode()))
		if err != nil {
			log.Println(err)
		}
		req.SetBasicAuth(botUsername, botPassword)
		req.Header.Set("content-type", "application/x-www-form-urlencoded")
		resp, err := zulipClient.Do(req)
		if err != nil {
			log.Panic(err)
		}
		defer resp.Body.Close()
		respBodyText, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Println(err)
		}
		log.Println(string(respBodyText))
	}
}

// this shuffles our recursers.
// TODO: source of randomness is time, but this runs at the
// same time each day. Is that ok?
func shuffle(slice []map[string]interface{}) []map[string]interface{} {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	ret := make([]map[string]interface{}, len(slice))
	perm := r.Perm(len(slice))
	for i, randIndex := range perm {
		ret[i] = slice[randIndex]
	}
	return ret
}
