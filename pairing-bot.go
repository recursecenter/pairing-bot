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

type zulipIncHook struct {
	BotEmail string `json:"bot_email"`
	Data     string `json:"data"`
	Token    string `json:"token"`
	Trigger  string `json:"trigger"`
	Message  struct {
		SenderID        int    `json:"sender_id"`
		SenderEmail     string `json:"sender_email"`
		SenderShortName string `json:"sender_short_name"`
	} `json:"message"`
}

type botResponse struct {
	Message string `json:"content"`
}

type recurser struct {
	SenderID        int
	SenderShortName string
	Subscribed      bool
}

func main() {
	http.HandleFunc("/webhooks", indexHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Defaulting to port %s", port)
	}

	log.Printf("Listening on port %s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	var userReq zulipIncHook
	err := json.NewDecoder(r.Body).Decode(&userReq)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	// Validate the Token value against ours to make sure request
	// is meant for us.

	datastoreClient, err := datastore.NewClient(ctx, "pairing-bot-238901")
	if err != nil {
		// Probably return 500 response, or "PairBot slipped on a banana peel."
	}

	key := datastore.NameKey("Recurser", "ZulipID", nil)

	zulipID := userReq.Message.SenderID
	firstName := userReq.Message.SenderShortName
	recurser := recurser{zulipID, firstName, false}
	datastoreClient.Put(ctx, key, recurser)
	log.Println(recurser.SenderID, zulipID)
	if err != nil {
		// Another banana peel.
	}

	res := botResponse{fmt.Sprintf("Added %v to our database!", firstName)}
	err = json.NewEncoder(w).Encode(res)
	if err != nil {
		log.Println(err)
	}
}
