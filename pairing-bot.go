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
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

// this is my real id (it's not really secret)
const marenID int = 215391
const maren string = `@_**Maren Beam (SP2'19)**`
const helpMessage string = `This is the help menu`
const subscribeMessage string = "Yay! You're now subscribed to Pairing Bot!\nCurrently, I'm set to find pair programming partners for you on **Mondays**, **Tuesdays**, **Wednesdays**, and **Thursdays**.\nYou can customize your schedule any time with `schedule`.\n\nThanks for signing up :)"
const unsubscribeMessage string = "You're unsubscribed!\nI won't find pairing partners for you unless you `subscribe`.\n\nBe well :)"

var writeError = fmt.Sprintf("Something went sideways while writing to the database. You should probably ping %v", maren)
var readError = fmt.Sprintf("Something went sideways while reading from the database. You should probably ping %v", maren)

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

/* func botAction(ctx context.Context, client *firestore.Client, userReq incomingJSON) (string, error) {
var response string
var err error
var recurser recurser

// create regex for removing internal whitespace in PM from user
space := regexp.MustCompile(`\s+`)

// split the PM into a slice of strings so we can look at it real good
pm := strings.Split(recurser.Data, " ")
// tell us whether the user is currently in the database
doc, err := client.Collection("recursers").Doc(recurser["id"].(string)).Get(ctx)
if err != nil && grpc.Code(err) != codes.NotFound {
	response = fmt.Sprintf(`Something went sideways while reading from the database. You should probably ping %v`, maren)
	return response, err
}
isSubscribed := doc.Exists()

switch {
// if the user string is "". This shouldn't be possible because zulip
// handles it but we should check anyway
case len(pm) == 0:
	response = `You didn't say anything, friend <3`

// if it's not a private message
case userReq.Trigger != "private_message":
	response = `plz don't @ me i only do pm's <3`

// if the first word of whatever they sent is "subscribe"
case pm[0] == "subscribe":
	if isSubscribed == false {
		_, err = client.Collection("recursers").Doc(recurser["id"].(string)).Set(ctx, recurser, firestore.MergeAll)
		if err != nil {
			response = fmt.Sprintf(`Something went sideways while writing to the database. You should probably ping %v`, maren)
			break
		}
		response = subscribeMessage
	} else {
		response = "You're already subscribed! Use `schedule` to set your schedule."
	}
// if they sent only the word "unsubscribe"
case pm[0] == "unsubscribe" && len(pm) == 1:
	if isSubscribed {
		_, err = client.Collection("recursers").Doc(recurser["id"].(string)).Delete(ctx)
		if err != nil {
			response = fmt.Sprintf(`Something went sideways while writing to the database. You should probably ping %v`, maren)
			break
		}
	}
	response = unsubscribeMessage

case pm[0] == "schedule":
	if isSubscribed == false {
		response = "You're not subscribed! First you need to sign up for Pairing Bot with `subscribe`"
		break
	}

	//for _, d := range pm[1:] {
	//	if val, ok := recurser[]
	//}

case pm[0] == "skip":

default:
	response = helpMessage
}

return response, err
} */
func dispatch(ctx context.Context, client *firestore.Client, cmd string, cmdArgs []string, userID string, userName string) (string, error) {
	var response string
	var err error
	var recurser = map[string]interface{}{
		"id":                 "string",
		"name":               "string",
		"isSkippingTomorrow": false,
		"schedule": map[string]interface{}{
			"monday":    false,
			"tuesday":   false,
			"wednesday": false,
			"thursday":  false,
			"friday":    false,
			"saturday":  false,
			"sunday":    false,
		},
	}

	// get the users "document" (database entry) out of firestore
	// we temporarily keep in in 'doc'
	doc, err := client.Collection("recursers").Doc(userID).Get(ctx)
	// this says "if theres and error, and if that error was not document-not-found"
	if err != nil && grpc.Code(err) != codes.NotFound {
		response = readError
		return response, err
	}
	// if there's a db entry, that means they were already subscribed to pairing bot
	// if there's not, they were not subscribed
	isSubscribed := doc.Exists()

	// if the user is in the database, get their current state into this map
	// also assign their zulip name to the name field, just in case it changed
	if isSubscribed {
		recurser = doc.Data()
		recurser["name"] = userName
	}
	// here's the actual actions. command input from
	// the user has already been sanitized, so we can
	// trust that cmd and cmdArgs only have valid stuff in them
	switch cmd {
	case "schedule":

	case "subscribe":
		if isSubscribed == false {
			recurser = map[string]interface{}{
				"id":                 userID,
				"name":               userName,
				"isSkippingTomorrow": false,
				"schedule": map[string]interface{}{
					"monday":    true,
					"tuesday":   true,
					"wednesday": true,
					"thursday":  true,
					"friday":    false,
					"saturday":  false,
					"sunday":    false,
				},
			}
			_, err = client.Collection("recursers").Doc(userID).Set(ctx, recurser)
			if err != nil {
				response = writeError
				break
			}
			response = subscribeMessage
		} else {
			response = "You're already subscribed! Use `schedule` to set your schedule."
		}

	case "unsubscribe":
		if isSubscribed {
			_, err = client.Collection("recursers").Doc(userID).Delete(ctx)
			if err != nil {
				response = writeError
				break
			}
		}
		response = unsubscribeMessage

	case "skip":
		recurser["isSkippingTomorrow"] = true
		_, err = client.Collection("recursers").Doc(userID).Set(ctx, recurser, firestore.MergeAll)
		if err != nil {
			response = writeError
			break
		}
		response = `Tomorrow: cancelled. I feel you. **I will not match you** for pairing tomorrow <3`

	case "unskip":
		recurser["isSkippingTomorrow"] = false
		_, err = client.Collection("recursers").Doc(userID).Set(ctx, recurser, firestore.MergeAll)
		if err != nil {
			response = writeError
			break
		}
		response = "Tomorrow: uncancelled! Heckin *yes*! **I will match you** for pairing tomorrow :)"

	case "status":
		// this particular days list is for sorting and printing the
		// schedule correctly, since it's stored in a map in all lowercase
		var daysList = []string{
			"Monday",
			"Tuesday",
			"Wednesday",
			"Thursday",
			"Friday",
			"Saturday",
			"Sunday"}

		// get their current name
		whoami := recurser["name"]

		// get skip status and prepare to write a sentence with it
		var skipStr string
		if recurser["isSkippingTomorrow"].(bool) {
			skipStr = "are"
		} else {
			skipStr = "are not"
		}

		// make a sorted list of their scheduke
		var schedule []string
		for _, v := range daysList {
			if recurser["schedule"].(map[string]interface{})[strings.ToLower(v)].(bool) {
				schedule = append(schedule, v)
			}
		}

		response = fmt.Sprintf("You are %v.\nYou are scheduled for pairing on %v.\nYou %v set to skip pairing tomorrow.", whoami, schedule, skipStr)

	case "help":
		response = helpMessage
	default:
		// this won't execute because all input has been sanitized
		// by parseCmd() and all cases are handled explicitly here
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
	if userReq.Trigger != "private_message" {
		err = responder.Encode(botResponse{`plz don't @ me i only do pm's <3`})
		if err != nil {
			log.Println(err)
		}
		return
	}
	// you *should* be able to throw any freakin string array at this thing and get back a valid command for dispatch()
	// if there are no commad arguments, cmdArgs will be nil
	cmd, cmdArgs, err := parseCmd(userReq.Data)
	if err != nil {
		log.Println(err)
	}
	// the tofu and potatoes right here y'all
	response, err := dispatch(ctx, client, cmd, cmdArgs, strconv.Itoa(userReq.Message.SenderID), userReq.Message.SenderFullName)
	if err != nil {
		log.Println(err)
	}
	err = responder.Encode(botResponse{response})
	if err != nil {
		log.Println(err)
	}
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

func parseCmd(cmdStr string) (string, []string, error) {
	var err error
	var cmdList = []string{
		"subscribe",
		"unsubscribe",
		"help",
		"schedule",
		"skip",
		"unskip",
		"status"}

	var daysList = []string{
		"monday",
		"tuesday",
		"wednesday",
		"thursday",
		"friday",
		"saturday",
		"sunday"}

	// convert the string to a slice
	// after this, we have a value "cmd" of type []string
	// where cmd[0] is the command and cmd[1:] are any arguments
	space := regexp.MustCompile(`\s+`)
	cmdStr = space.ReplaceAllString(cmdStr, ` `)
	cmdStr = strings.TrimSpace(cmdStr)
	cmdStr = strings.ToLower(cmdStr)
	cmd := strings.Split(cmdStr, ` `)

	// Big validation logic -- hellooo darkness my old frieeend
	switch {
	// if there's nothing in the command string srray
	case len(cmd) == 0:
		err = errors.New("the user-issued command was blank")
		return "help", nil, err

	// if there's a valid command and if there's no arguments
	case contains(cmdList, cmd[0]) && len(cmd) == 1:
		if cmd[0] == "schedule" || cmd[0] == "skip" || cmd[0] == "unskip" {
			err = errors.New("the user issued a command without args, but it required args")
			return "help", nil, err
		}
		return cmd[0], nil, err

	// if there's a valid command and there's some arguments
	case contains(cmdList, cmd[0]) && len(cmd) > 1:
		switch {
		case cmd[0] == "subscribe" || cmd[0] == "unsubscribe" || cmd[0] == "help" || cmd[0] == "status":
			err = errors.New("the user issued a command with args, but it disallowed args")
			return "help", nil, err
		case cmd[0] == "skip" && len(cmd) != 2 && cmd[1] != "tomorrow":
			err = errors.New("the user issued SKIP with malformed arguments")
			return "help", nil, err
		case cmd[0] == "unskip" && len(cmd) != 2 && cmd[1] != "tomorrow":
			err = errors.New("the user issued UNSKIP with malformed arguments")
			return "help", nil, err
		case cmd[0] == "schedule":
			for _, v := range cmd[1:] {
				if contains(daysList, v) == false {
					err = errors.New("the user issued SCHEDULE with malformed arguments")
					return "help", nil, err
				}
			}
			fallthrough
		default:
			return cmd[0], cmd[1:], err
		}

	// if there's not a valid command
	default:
		err = errors.New("the user-issued command wasn't valid")
		return "help", nil, err
	}
}

func contains(list []string, cmd string) bool {
	for _, v := range list {
		if v == cmd {
			return true
		}
	}
	return false
}

func nope(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}
