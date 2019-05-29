package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"cloud.google.com/go/firestore"
)

const mcb int = 215391

var err error

type incomingJSON struct {
	BotEmail string `json:"bot_email"`
	Data     string `json:"data"`
	Token    string `json:"token"`
	Trigger  string `json:"trigger"`
	Message  struct {
		SenderID       int    `json:"sender_id"`
		SenderEmail    string `json:"sender_email"`
		SenderFullName string `json:"sender_full_name"`
	} `json:"message"`
}

type botResponse struct {
	Message string `json:"content"`
}

type recurser struct {
	ID   string `firestore:"id"`
	Name string `firestore:"name"`
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
		err = respond("Oof, Zulip may have just sent me invalid JSON.", w)
		if err != nil {
			log.Println(err)
		}
		return
	}

	// validate RC's Zulip token
	// if it doesn't validate, we don't know this person
	// and thus don't give them anything
	err = validateRequest(userRequest)
	if err != nil {
		log.Println(err)
		http.NotFound(w, r)
		return
	}

	// Act on a user request. This both parses and acts and responds
	// Currently a bit of a catch-all. Candidate for breaking
	// up later.
	response, err := touchdb(userRequest)
	if err != nil {
		log.Println(err)
		err = respond(response, w)
		if err != nil {
			log.Println(err)
		}
		return
	}

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		log.Println("Bot attempted to respond but failed.")
	}
}

func validateRequest(userRequest incomingJSON) error {
	//validate the request
	return nil
}

func touchdb(userRequest incomingJSON) (string, error) {
	// if it's not Maren messaging the bot, just say uwu
	if userRequest.Message.SenderID != mcb {
		return `uwu`, nil
	}

	// Get set up to talk to the Firestore database
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, "pairing-bot-238901")
	if err != nil {
		return `error!`, err
	}

	// Get the data we need about the person making the request
	recurser := recurser{
		ID:   strconv.Itoa(userRequest.Message.SenderID),
		Name: userRequest.Message.SenderFullName,
	}
	// key := firestore.NameKey("Recurser", "ZulipID", nil)

	_, err = client.Collection("recursers").Doc(recurser.ID).Set(ctx, recurser, firestore.MergeAll)
	if err != nil {
		return `Something went sideways while writing to the Database`, err
	}

	response := fmt.Sprintf("Added %v to our database!", recurser.Name)
	return response, nil
}

func respond(response string, w http.ResponseWriter) error {
	err = json.NewEncoder(w).Encode(botResponse{response})
	if err != nil {
		return err
	}
	return nil
}

// It's alive! The app starts here.
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
