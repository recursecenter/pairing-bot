package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"cloud.google.com/go/firestore"
)

// this is my real id (it's not really secret)
const marenID int = 215391
const maren string = `@_**Maren Beam (SP2'19)**`
const helpMessage string = `This is the help menu`
const subscribedMessage string = "Yay! You're now subscribed to Pairing Bot!\nCurrently, I'm set to find pair programming partners for you on Mondays, Tuesdays, Wednesdays, and Thursdays.\nYou can customize your schedule any time with `schedule`.\n\nThanks for signing up :)"

// this is my wrong ID, for testing how pairing-bot
// responds to other users
// const marenID int = 215393

// This is a struct that gets only what
// we need from the incoming JSON payload
type incomingJSON struct {
	Data    string `json:"data"`
	Token   string `json:"token"`
	Trigger string `json:"trigger"`
	Message struct {
		SenderID       int    `json:"sender_id"`
		SenderFullName string `json:"sender_full_name"`
	} `json:"message"`
}

// Zulip has to get JSON back from the bot,
// this does that. An empty message field stops
// zulip from throwing an error at the user that
// messaged the bot, but doesn't send a response
type botResponse struct {
	Message string `json:"content"`
}

var weekdays = map[string]int{
	"monday":    0,
	"tuesday":   1,
	"wednesday": 2,
	"thursday":  3,
	"friday":    4,
	"saturday":  5,
	"sunday":    6,
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
		log.Println("Something weird happend trying to read the auth token from the database")
		return userReq, err
	}
	token := doc.Data()
	if userReq.Token != token["value"] {
		http.NotFound(w, r)
		return userReq, errors.New("unauthorized interaction attempt")
	}
	return userReq, err
}

func botAction(ctx context.Context, client *firestore.Client, userReq incomingJSON) (string, error) {
	var response string
	var err error

	// create regex for removing internal whitespace
	// in PM from user
	space := regexp.MustCompile(`\s+`)
	// make the lil' recurser map object. Mapject?
	recurser := map[string]interface{}{
		"id":      strconv.Itoa(userReq.Message.SenderID),
		"name":    userReq.Message.SenderFullName,
		"message": strings.ToLower(strings.TrimSpace(space.ReplaceAllString(userReq.Data, ` `))),
	}

	// split the PM into a slice  of strings so we can look at it real good
	pm := strings.Split(recurser["message"].(string), " ")
	// tell us whether the user is currently in the database
	doc, err := client.Collection("recursers").Doc(recurser["id"].(string)).Get(ctx)
	if err != nil && grpc.Code(err) != codes.NotFound {
		response = fmt.Sprintf(`Something went sideways while reading from the database. You should probably ping %v`, maren)
		return response, err
	}
	isSubscribed := doc.Exists()

	switch {
	case len(pm) == 0:
		response = `You didn't say anything, friend <3`

	case userReq.Trigger != "private_message":
		response = `plz don't @ me i only do pm's <3`

	case pm[0] == "subscribe":
		if isSubscribed == false {
			_, err = client.Collection("recursers").Doc(recurser["id"].(string)).Set(ctx, recurser, firestore.MergeAll)
			if err != nil {
				response = fmt.Sprintf(`Something went sideways while writing to the database. You should probably ping %v`, maren)
				break
			}
			response = subscribedMessage
		} else {
			response = "You're already subscribed! Use `schedule` to set your schedule."
		}

	case pm[0] == "unsubscribe":
		if isSubscribed {
			_, err = client.Collection("recursers").Doc(recurser["id"].(string)).Delete(ctx)
			if err != nil {
				response = fmt.Sprintf(`Something went sideways while writing to the database. You should probably ping %v`, maren)
				break
			}
		}
		response = "You're unsubscribed! I won't find you pairing partners unless you `subscribe` again. Be well :)"

	case pm[0] == "schedule":

	case pm[0] == "skip":

	default:
		response = helpMessage
	}

	return response, err
}

func handle(w http.ResponseWriter, r *http.Request) {
	responder := json.NewEncoder(w)

	ctx := context.Background()
	client, err := firestore.NewClient(ctx, "pairing-bot-242820")
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
	// this responds uwu and quits if it's not me
	if userReq.Message.SenderID != marenID {
		err = responder.Encode(botResponse{`uwu`})
		if err != nil {
			log.Println(err)
		}
		return
	}

	response, err := botAction(ctx, client, userReq)
	if err != nil {
		log.Println(err)
	}
	err = responder.Encode(botResponse{response})
	if err != nil {
		log.Println(err)
	}
	return
}

func nope(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}

// It's alive! The application starts here.
func main() {
	http.HandleFunc("/", nope)
	http.HandleFunc("/webhooks", handle)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Defaulting to port %s", port)
	}

	log.Printf("Listening on port %s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}
