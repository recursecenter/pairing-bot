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

	"cloud.google.com/go/firestore"
)

// this is my real id (it's not really secret)
// const marenID int = 215391
const maren string = `@_**Maren Beam (SP2'19)**`

// this is my wrong ID, for testing how pairing-bot
// responds to other users
const marenID int = 215393

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
	recurser := map[string]string{
		"id":      strconv.Itoa(userReq.Message.SenderID),
		"name":    userReq.Message.SenderFullName,
		"message": strings.ToLower(strings.TrimSpace(space.ReplaceAllString(userReq.Data, ` `))),
	}

	// split the PM into a slice  of strings so we can look at it real good
	pm := strings.Split(recurser["message"], " ")

	switch {
	case len(pm) == 0:
		response = `You didn't say anything, friend <3`

	case userReq.Trigger != "private_message":
		response = `plz don't @ me i only do pm's <3`

	case pm[0] == "subscribe":
		// check if the user is already subscribed
		doc, err := client.Collection("recursers").Doc(recurser["id"]).Get(ctx)
		if err != nil {
			response = fmt.Sprintf(`Something went sideways while reading from the database. You should probably ping %v`, maren)
			break
		}
		if doc.Exists() == false {
			_, err = client.Collection("recursers").Doc(recurser["id"]).Set(ctx, recurser, firestore.MergeAll)
			if err != nil {
				response = fmt.Sprintf(`Something went sideways while writing to the database. You should probably ping %v`, maren)
				break
			}
			response = fmt.Sprintf("Yay! You're now subscribed to Pairing Bot!\nI'll find you a pair programming partner on Monday through Thursday, unless you set a new schedule with `schedule`.\nThanks for signing up, %v :)", recurser["name"])
		} else {
			response = "You're already subscribed! Use `schedule` to set your schedule."
		}

	case pm[0] == "unsubscribe":
	case pm[0] == "schedule":
	case pm[0] == "status":
	case pm[0] == "skip":
	default:
		response = `This is the help menu`
	}

	return response, err
}

func help() (string, error) {
	return `This is the help menu`, nil
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

	// if it was a private message do this
	// TODO: i'd like to handle this differently
	if userReq.Trigger != "private_message" {
		err = responder.Encode(botResponse{`plz don't @ me i only do pm's <3`})
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
