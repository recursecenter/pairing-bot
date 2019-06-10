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
type command string

// Any incoming http request is handled here
func sanitize(w http.ResponseWriter, r *http.Request) {

	responder := json.NewEncoder(w)

	// 404 for anything other than /webhooks
	if r.URL.Path != "/webhooks" {
		http.NotFound(w, r)
		return
	}

	// Look at the incoming webhook and slurp up the JSON
	// Error if the JSON from Zulip istelf is bad
	var userRequest incomingJSON
	err := json.NewDecoder(r.Body).Decode(&userRequest)
	if err != nil {
		log.Println(err)
		http.NotFound(w, r)
		return
	}

	// validate the bot's zulip-assigned token
	// if it doesn't validate, we don't know this
	// person so don't give them anything
	// TODO -- Does this really need the whole userRequest object?
	err = validateRequest(userRequest)
	if err != nil {
		log.Println(err)
		http.NotFound(w, r)
		return
	}

	// Check if it's me
	// This is just for testing
	if userRequest.Message.SenderID != mcb {
		err = responder.Encode(botResponse{`uwu`})
		if err != nil {
			log.Println(err)
		}
		return
	}

	// if the bot was @-mentioned, do this
	if userRequest.Trigger != "private_message" {
		err = responder.Encode(botResponse{`plz don't @ me i only do pm's <3`})
		if err != nil {
			log.Println(err)
		}
		return
	}

	//
	// Act on a user request. This parses and acts and responds
	// Currently a bit of a catch-all. Candidate for breaking
	// up later.
	response, err := touchdb(userRequest)
	if err != nil {
		log.Println(err)
	}
	err = responder.Encode(botResponse{response})
	if err != nil {
		log.Println(err)
	}
	return
}

// validate the request
// The token it's checking was manually put into the database
// Afaict that was the suggested GAE way to keep it out of version control
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

	pm := strings.Split(recurser.Message, " ")
	if len(pm) != 0 {
		response, err := help()
		if err != nil {
			return "I failed at parsing your message", err
		}
		return response, nil
	}

	// this runs the command based on user input and the
	// existing list (map) of commands
	/*
		for key, value := range commands {

		}
	*/
	return "", nil
}

func help() (string, error) {
	return `This is the help menu`, nil
}

func subscribe(recurser recurser, pm string) (string, error) {
	// Get set up to talk to the Firestore database
	// this is just firestore boilerplate
	// Tthis gets repeated a lot, but I didn't want
	// to pass all this stuff to every single function either
	// ugh i need code review so bad plz
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, "pairing-bot-242820")
	if err != nil {
		return `There was a spooky cloud error`, err
	}

	// Subscribe the user
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
	return fmt.Sprintf("%v is now subscribed to Pairing Bot! Thanks for signing up, you :)", recurser.Name), nil
}

/*
func status(recurser recurser, pm string) (string, error) {
	return "", nil
}

func skiptomorrow(recurser recurser, pm string) (string, error) {
	return "", nil
}

func unsubscribe(recurser recurser, pm string) (string, error) {
	return "", nil
}

func schedule(recurser recurser, pm string) (string, error) {
	return "", nil
}
*/
func handle(w http.ResponseWriter, r *http.Request) {
	sanitize(w, r)
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
