package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"cloud.google.com/go/firestore"
)

// this is my real id (it's not really secret)
const mcb int = 215391

// this is my wrong ID, for testing how pairing-bot
// responds to other users
// const mcb int = 215393

var err error

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

type recurser struct {
	ID      string `firestore:"id,omitempty"`
	Name    string `firestore:"name,omitempty"`
	Message string `firestore:"message,omitempty"`
}

// starting to figure out how to map out the
// commands that the user can send. This will
// probably change
var commands = map[string]string{
	"sub":    "subscribe",
	"help":   "help",
	"status": "status",
	"skip":   "skip tomorrow",
	"unsub":  "unsubscribe",
}

// Any incoming http request is handled here
func handle(w http.ResponseWriter, r *http.Request) {

	// 404 for anything other than /webhooks
	if r.URL.Path != "/webhooks" {
		http.NotFound(w, r)
		return
	}

	// Look at the incoming webhook and slurp up the JSON
	// Error if the JSON from Zulip istelf is bad
	var userRequest incomingJSON
	err = json.NewDecoder(r.Body).Decode(&userRequest)
	if err != nil {
		log.Println(err)
		http.NotFound(w, r)
		return
	}

	// validate the bot's zulip-assigned token
	// if it doesn't validate, we don't know this
	// person so don't give them anything
	err = validateRequest(userRequest)
	if err != nil {
		log.Println(err)
		http.NotFound(w, r)
		return
	}

	// Check if it's me
	// This is just for testing
	if userRequest.Message.SenderID != mcb {
		err = respond(`uwu`, w)
		if err != nil {
			log.Println(err)
		}
		return
	}

	// if the bot was @-mentioned, do this
	if userRequest.Trigger != "private_message" {
		err = respond(`plz don't @ me i only do pm's <3`, w)
		if err != nil {
			log.Println(err)
		}
		return
	}

	// Act on a user request. This parses and acts and responds
	// Currently a bit of a catch-all. Candidate for breaking
	// up later.
	response, err := touchdb(userRequest)
	if err != nil {
		log.Println(err)
	}
	err = respond(response, w)
	if err != nil {
		log.Println(err)
	}
	return
}

//validate the request
func validateRequest(userRequest incomingJSON) error {
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, "pairing-bot-242820")
	if err != nil {
		return err
	}
	document, err := client.Collection("botauth").Doc("token").Get(ctx)
	token := document.Data()
	if userRequest.Token == token["value"] {
		return nil
	}
	return errors.New("unauthorized interaction attempt")
}

func touchdb(userRequest incomingJSON) (string, error) {
	// Get set up to talk to the Firestore database
	// this is just firestore boilerplate
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, "pairing-bot-242820")
	if err != nil {
		return `There was a spooky cloud error`, err
	}

	// Get the data we need about the user making the request
	// into an object in program memory
	// All we need is SenderID, which is a unique zulip account
	// ID that never changes, SenderFullName, which is
	// the user's full name on zulip (including their batch),
	// and Data, which is the contents of the private message that
	// the user sent to pairing bot.
	recurser := recurser{
		ID:      strconv.Itoa(userRequest.Message.SenderID),
		Name:    userRequest.Message.SenderFullName,
		Message: strings.ToLower(strings.TrimSpace(userRequest.Data)),
	}

	// This is a little sloppy, but works. This just  overwrites
	// all the fields for the db entry for this recurser with
	// new data, every time. There is a better way to do this with
	// "firestore.MergeAll", which only overwrites data in fields
	// with changed data, however using it requires declaring the type
	// with a map rather than a struct, which I didn't want to do
	// because it doesn't make as much sense to me. This is worth looking
	// into in the future.
	_, err = client.Collection("recursers").Doc(recurser.ID).Set(ctx, recurser)
	if err != nil {
		return `Something went sideways while writing to the Database`, err
	}
	return fmt.Sprintf("Added %v to our database!", recurser.Name), nil
}

// I found that I was writing this out a lot, so I broke it
// out into a function. I'm not sure if it was the best idea,
// because there's still a bunch of error handling that the
// caller has to do which looks just as messy as before, but that
// could probably be handled with some custom error types a la
// https://www.innoq.com/en/blog/golang-errors-monads/
func respond(response string, w http.ResponseWriter) error {
	err = json.NewEncoder(w).Encode(botResponse{response})
	if err != nil {
		return err
	}
	return nil
}

// It's alive! The application starts here.
func main() {
	http.HandleFunc("/", handle)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Defaulting to port %s", port)
	}

	log.Printf("Listening on port %s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}
