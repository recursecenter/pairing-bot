package main

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const helpMessage string = "**How to use Pairing Bot:**\n* `subscribe` to start getting matched with other Pairing Bot users for pair programming\n* `schedule monday wednesday friday` to set your weekly pairing schedule\n  * In this example, I've been set to find pairing partners for you on every Monday, Wednesday, and Friday\n  * You can schedule pairing for any combination of days in the week\n* `skip tomorrow` to skip pairing tomorrow\n  * This is valid until matches go out at 04:00 UTC\n* `unskip tomorrow` to undo skipping tomorrow\n* `status` to show your current schedule, skip status, and name\n* `unsubscribe` to stop getting matched entirely\n\nIf you've found a bug, please [submit an issue on github](https://github.com/thwidge/pairing-bot/issues)!"
const subscribeMessage string = "Yay! You're now subscribed to Pairing Bot!\nCurrently, I'm set to find pair programming partners for you on **Mondays**, **Tuesdays**, **Wednesdays**, **Thursdays**, and **Fridays**.\nYou can customize your schedule any time with `schedule` :)"
const unsubscribeMessage string = "You're unsubscribed!\nI won't find pairing partners for you unless you `subscribe`.\n\nBe well :)"
const notSubscribedMessage string = "You're not subscribed to Pairing Bot <3"

var writeErrorMessage = fmt.Sprintf("Something went sideways while writing to the database. You should probably ping %v", owner)
var readErrorMessage = fmt.Sprintf("Something went sideways while reading from the database. You should probably ping %v", owner)

func dispatch(ctx context.Context, client *firestore.Client, cmd string, cmdArgs []string, userID string, userEmail string, userName string) (string, error) {
	var response string
	var err error
	var recurser = map[string]interface{}{
		"id":                 "string",
		"name":               "string",
		"email":              "string",
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
	// we temporarily keep it in 'doc'
	doc, err := client.Collection("recursers").Doc(userID).Get(ctx)
	// this says "if there's an error, and if that error was not document-not-found"
	if err != nil && status.Code(err) != codes.NotFound {
		response = readErrorMessage
		return response, err
	}
	// if there's a db entry, that means they were already subscribed to pairing bot
	// if there's not, they were not subscribed
	isSubscribed := doc.Exists()

	// if the user is in the database, get their current state into this map
	// also assign their zulip name to the name field, just in case it changed
	// also assign their email, for the same reason
	if isSubscribed {
		recurser = doc.Data()
		recurser["name"] = userName
		recurser["email"] = userEmail
	}
	// here's the actual actions. command input from
	// the user has already been sanitized, so we can
	// trust that cmd and cmdArgs only have valid stuff in them
	switch cmd {
	case "schedule":
		if !isSubscribed {
			response = notSubscribedMessage
			break
		}
		// create a new blank schedule
		var newSchedule = map[string]interface{}{
			"monday":    false,
			"tuesday":   false,
			"wednesday": false,
			"thursday":  false,
			"friday":    false,
			"saturday":  false,
			"sunday":    false,
		}
		// populate it with the new days they want to pair on
		for _, day := range cmdArgs {
			newSchedule[day] = true
		}
		// put it in the database
		recurser["schedule"] = newSchedule
		_, err = client.Collection("recursers").Doc(userID).Set(ctx, recurser, firestore.MergeAll)
		if err != nil {
			response = writeErrorMessage
			break
		}
		response = "Awesome, your new schedule's been set! You can check it with `status`."

	case "subscribe":
		if isSubscribed {
			response = "You're already subscribed! Use `schedule` to set your schedule."
			break
		}

		// recurser isn't really a type, because we're using maps
		// and not struct. but we're using it *as* a type,
		// and this is the closest thing to a definition that occurs
		recurser = map[string]interface{}{
			"id":                 userID,
			"name":               userName,
			"email":              userEmail,
			"isSkippingTomorrow": false,
			"schedule": map[string]interface{}{
				"monday":    true,
				"tuesday":   true,
				"wednesday": true,
				"thursday":  true,
				"friday":    true,
				"saturday":  false,
				"sunday":    false,
			},
		}
		_, err = client.Collection("recursers").Doc(userID).Set(ctx, recurser)
		if err != nil {
			response = writeErrorMessage
			break
		}
		response = subscribeMessage

	case "unsubscribe":
		if !isSubscribed {
			response = notSubscribedMessage
			break
		}
		_, err = client.Collection("recursers").Doc(userID).Delete(ctx)
		if err != nil {
			response = writeErrorMessage
			break
		}
		response = unsubscribeMessage

	case "skip":
		if !isSubscribed {
			response = notSubscribedMessage
			break
		}
		recurser["isSkippingTomorrow"] = true
		_, err = client.Collection("recursers").Doc(userID).Set(ctx, recurser, firestore.MergeAll)
		if err != nil {
			response = writeErrorMessage
			break
		}
		response = `Tomorrow: cancelled. I feel you. **I will not match you** for pairing tomorrow <3`

	case "unskip":
		if !isSubscribed {
			response = notSubscribedMessage
			break
		}
		recurser["isSkippingTomorrow"] = false
		_, err = client.Collection("recursers").Doc(userID).Set(ctx, recurser, firestore.MergeAll)
		if err != nil {
			response = writeErrorMessage
			break
		}
		response = "Tomorrow: uncancelled! Heckin *yes*! **I will match you** for pairing tomorrow :)"

	case "status":
		if !isSubscribed {
			response = notSubscribedMessage
			break
		}
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
			skipStr = " "
		} else {
			skipStr = " not "
		}

		// make a sorted list of their schedule
		var schedule []string
		for _, day := range daysList {
			// this line is a little wild, sorry. it looks so weird because we
			// have to do type assertion on both interface types
			if recurser["schedule"].(map[string]interface{})[strings.ToLower(day)].(bool) {
				schedule = append(schedule, day)
			}
		}
		// make a lil nice-lookin schedule string
		var scheduleStr string
		for i := range schedule[:len(schedule)-1] {
			scheduleStr += schedule[i] + "s, "
		}
		if len(schedule) > 1 {
			scheduleStr += "and " + schedule[len(schedule)-1] + "s"
		} else if len(schedule) == 1 {
			scheduleStr += schedule[0] + "s"
		}

		response = fmt.Sprintf("* You're %v\n* You're scheduled for pairing on **%v**\n* **You're%vset to skip** pairing tomorrow", whoami, scheduleStr, skipStr)

	case "help":
		response = helpMessage
	default:
		// this won't execute because all input has been sanitized
		// by parseCmd() and all cases are handled explicitly above
	}
	return response, err
}
