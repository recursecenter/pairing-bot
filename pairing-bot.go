package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/datastore"
)

const mcb int = 215391

var err error

type incomingHook struct {
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
	SenderID       int
	SenderFullName string
	Subscribed     bool
}

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

func handle(w http.ResponseWriter, r *http.Request) {
	var userRequest incomingHook

	// 404 for anything other than /webhooks
	if r.URL.Path != "/webhooks" {
		http.NotFound(w, r)
		return
	}

	// Look at the incoming webhook and slurp up the JSON
	// Error is the POST request from Zulip istelf is bad
	err = json.NewDecoder(r.Body).Decode(&userRequest)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	// validate RC's Zulip instance token
	err = validateRequest(userRequest)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Act on a user request. This both parses and acts and responds
	// Currently a bit of a catch-all. Candidate for breaking
	// up later.
	response, err := touchdb(userRequest)
	if err != nil {
		err = json.NewEncoder(w).Encode(botResponse{`Something wrong with your command fren`})
		if err != nil {
			log.Println(err)
			return
		}
		return
	}

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		log.Println("Bot attempted to respond but failed.")
	}
}

func validateRequest(userRequest incomingHook) error {
	//validate the request
	return nil
}

func touchdb(userRequest incomingHook) (botResponse, error) {
	// if it's not Maren messaging the bot, just say uwu
	if userRequest.Message.SenderID != mcb {
		return botResponse{`uwu`}, nil
	}

	// Figure out what this does
	ctx := context.Background()

	datastoreClient, err := datastore.NewClient(ctx, "pairing-bot-238901")
	if err != nil {
		return botResponse{`error!`}, err
	}

	key := datastore.NameKey("Recurser", "ZulipID", nil)

	zulipID := userRequest.Message.SenderID
	fullName := userRequest.Message.SenderFullName
	recurser := recurser{zulipID, fullName, false}
	datastoreClient.Put(ctx, key, recurser)

	response := botResponse{fmt.Sprintf("Added %v to our database!", fullName)}
	return response, nil
}
