package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"
)

const owner string = `@_**Maren Beam (SP2'19)**`
const oddOneOutMessage string = "OK this is awkward.\nThere were an odd number of people in the match-set today, which means that one person couldn't get paired. Unfortunately, it was you -- I'm really sorry :(\nI promise it's not personal, it was very much random. Hopefully this doesn't happen again too soon. Enjoy your day! <3"
const matchedMessage = "Hi you two! You've been matched for pairing :)\n\nHave fun!"
const offboardedMessage = "Hi! You've been unsubscribed from Pairing Bot.\n\nThis happens at the end of every batch, when everyone is offboarded even if they're still in batch. If you'd like to re-subscribe, just send me a message that says `subscribe`.\n\nBe well! :)"

var maintenanceMode = false

// this is the "id" field from zulip, and is a permanent user ID that's not secret
// Pairing Bot's owner can add their ID here for testing. ctrl+f "ownerID" to see where it's used
const ownerID = "215391"

type PairingLogic struct {
	rdb RecurserDB
	adb APIAuthDB
	ur  userRequest
	un  userNotification
}

func (pl *PairingLogic) handle(w http.ResponseWriter, r *http.Request) {
	responder := json.NewEncoder(w)

	// check and authorize the incoming request
	// observation: we only validate requests for /webhooks, i.e. user input through zulip

	ctx := r.Context()
	err := pl.ur.validateJSON(r)
	if err != nil {
		http.NotFound(w, r)
	}

	botAuth, err := pl.adb.GetKey(ctx, "botauth", "token")
	if err != nil {
		log.Println("Something weird happened trying to read the auth token from the database")
	}

	if !pl.ur.validateAuthCreds(botAuth) {
		http.NotFound(w, r)
	}

	intro := pl.ur.validateInteractionType()
	if intro != nil {
		err = responder.Encode(intro)
		if err != nil {
			log.Println(err)
		}
		return
	}

	ignore := pl.ur.ignoreInteractionType()
	if ignore != nil {
		err = responder.Encode(ignore)
		if err != nil {
			log.Println(err)
		}
		return
	}

	userData := pl.ur.extractUserData()

	// for testing only
	// this responds with a maintenance message and quits if the request is coming from anyone other than the owner
	if maintenanceMode {
		if userData.userID != ownerID {
			err = responder.Encode(botResponse{`pairing bot is down for maintenance`})
			if err != nil {
				log.Println(err)
			}
			return
		}
	}

	// you *should* be able to throw any string at this thing and get back a valid command for dispatch()
	// if there are no commad arguments, cmdArgs will be nil
	cmd, cmdArgs, err := pl.ur.sanitizeUserInput()
	if err != nil {
		log.Println(err)
	}

	// the tofu and potatoes right here y'all

	response, err := dispatch(ctx, pl, cmd, cmdArgs, userData.userID, userData.userEmail, userData.userName)
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
func (pl *PairingLogic) match(w http.ResponseWriter, r *http.Request) {
	// Check that the request is originating from within app engine
	// https://cloud.google.com/appengine/docs/flexible/go/scheduling-jobs-with-cron-yaml#validating_cron_requests
	if r.Header.Get("X-Appengine-Cron") != "true" {
		http.NotFound(w, r)
		return
	}

	ctx := r.Context()

	recursersList, err := pl.rdb.ListPairingTomorrow(ctx)
	if err != nil {
		log.Printf("Could not get list of recursers from DB: %s\n", err)
	}

	skippersList, err := pl.rdb.ListSkippingTomorrow(ctx)
	if err != nil {
		log.Printf("Could not get list of skippers from DB: %s\n", err)
	}

	// get everyone who was set to skip today and set them back to isSkippingTomorrow = false
	for _, skipper := range skippersList {
		err := pl.rdb.UnsetSkippingTomorrow(ctx, skipper)
		if err != nil {
			log.Printf("Could not unset skipping for recurser %v: %s\n", skipper.id, err)
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
	botPassword, err := pl.adb.GetKey(ctx, "apiauth", "key")
	if err != nil {
		log.Println("Something weird happened trying to read the auth token from the database")
	}

	// if there's an odd number today, message the last person in the list
	// and tell them they don't get a match today, then knock them off the list
	if len(recursersList)%2 != 0 {
		recurser := recursersList[len(recursersList)-1]
		recursersList = recursersList[:len(recursersList)-1]
		log.Println("Someone was the odd-one-out today")

		err := pl.un.sendUserMessage(ctx, botPassword, recurser.email, oddOneOutMessage)
		if err != nil {
			log.Printf("Error when trying to send oddOneOut message to %s: %s\n", recurser.email, err)
		}

	}

	for i := 0; i < len(recursersList); i += 2 {

		emails := recursersList[i].email + ", " + recursersList[i+1].email
		err := pl.un.sendUserMessage(ctx, botPassword, emails, matchedMessage)
		if err != nil {
			log.Printf("Error when trying to send matchedMessage to %s: %s\n", emails, err)
		}
		log.Println(recursersList[i].email, "was", "matched", "with", recursersList[i+1].email)
	}
}

func (pl *PairingLogic) endofbatch(w http.ResponseWriter, r *http.Request) {
	// Check that the request is originating from within app engine
	// https://cloud.google.com/appengine/docs/flexible/go/scheduling-jobs-with-cron-yaml#validating_cron_requests
	if r.Header.Get("X-Appengine-Cron") != "true" {
		http.NotFound(w, r)
		return
	}

	// getting all the recursers
	ctx := r.Context()
	recursersList, err := pl.rdb.GetAllUsers(ctx)
	if err != nil {
		log.Panic(err)
	}

	// message and offboard everyone (delete them from the database)

	botPassword, err := pl.adb.GetKey(ctx, "apiauth", "key")
	if err != nil {
		log.Println("Something weird happened trying to read the auth token from the database")
	}

	for i := 0; i < len(recursersList); i++ {

		recurserID := recursersList[i].id
		recurserEmail := recursersList[i].email
		var message string

		err = pl.rdb.Delete(ctx, recurserID)
		if err != nil {
			log.Println(err)
			message = fmt.Sprintf("Uh oh, I was trying to offboard you since it's the end of batch, but something went wrong. Consider messaging %v to let them know this happened.", owner)
		} else {
			log.Println("A user was offboarded because it's the end of a batch.")
			message = offboardedMessage
		}

		err := pl.un.sendUserMessage(ctx, botPassword, recurserEmail, message)
		if err != nil {
			log.Printf("Error when trying to send offboarding message to %s: %s\n", recurserEmail, err)
		}
	}
}

// this shuffles our recursers.
func shuffle(slice []Recurser) []Recurser {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	ret := make([]Recurser, len(slice))
	perm := r.Perm(len(slice))
	for i, randIndex := range perm {
		ret[i] = slice[randIndex]
	}
	return ret
}
