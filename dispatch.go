package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

func (pl *PairingLogic) dispatch(ctx context.Context, cmd string, cmdArgs []string, userID int64, userEmail string, userName string) (string, error) {
	var response string
	var err error

	rec, err := pl.rdb.GetByUserID(ctx, userID, userEmail, userName)
	if err != nil {
		response = readErrorMessage
		return response, err
	}

	isSubscribed := rec.IsSubscribed

	// here's the actual actions. command input from
	// the user input has already been sanitized, so we can
	// trust that cmd and cmdArgs only have valid stuff in them
	switch cmd {
	case "schedule":
		if !isSubscribed {
			response = notSubscribedMessage
			break
		}

		rec.Schedule = newSchedule(cmdArgs)

		if err = pl.rdb.Set(ctx, userID, rec); err != nil {
			response = writeErrorMessage
			break
		}
		response = "Awesome, your new schedule's been set! You can check it with `status`."

	case "subscribe":
		if isSubscribed {
			response = "You're already subscribed! Use `schedule` to set your schedule."
			break
		}

		rec.CurrentlyAtRC, err = pl.recurse.IsCurrentlyAtRC(ctx, userID)
		if err != nil {
			log.Printf("Could not read currently-at-RC data from database: %s", err)
			response = writeErrorMessage
			break
		}

		if err = pl.rdb.Set(ctx, userID, rec); err != nil {
			log.Printf("Could not update from database: %s", err)
			response = writeErrorMessage
			break
		}
		response = subscribeMessage

	case "unsubscribe":
		if !isSubscribed {
			response = notSubscribedMessage
			break
		}

		if err := pl.rdb.Delete(ctx, userID); err != nil {
			response = writeErrorMessage
			break
		}
		response = unsubscribeMessage

	case "skip":
		if !isSubscribed {
			response = notSubscribedMessage
			break
		}

		rec.IsSkippingTomorrow = true

		if err := pl.rdb.Set(ctx, userID, rec); err != nil {
			response = writeErrorMessage
			break
		}
		response = `Tomorrow: cancelled. I feel you. **I will not match you** for pairing tomorrow <3`

	case "unskip":
		if !isSubscribed {
			response = notSubscribedMessage
			break
		}
		rec.IsSkippingTomorrow = false

		if err := pl.rdb.Set(ctx, userID, rec); err != nil {
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
			"Sunday",
		}

		// get their current name
		whoami := rec.Name

		// get skip status and prepare to write a sentence with it
		var skipStr string
		if rec.IsSkippingTomorrow {
			skipStr = " "
		} else {
			skipStr = " not "
		}

		// make a sorted list of their schedule
		var schedule []string
		for _, day := range daysList {
			// this line is a little wild, sorry. it looks so weird because we
			// have to do type assertion on both interface types
			if rec.Schedule[strings.ToLower(day)] {
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
	case "add-review":
		reviewContent := cmdArgs[0]

		currentTimestamp := time.Now().Unix()

		err = pl.revdb.Insert(ctx, Review{
			content:   reviewContent,
			timestamp: int(currentTimestamp),
			email:     userEmail,
		})

		if err != nil {
			log.Println("Encountered an error when trying to save a review: ", err)
			response = writeErrorMessage
			break
		}

		response = "Thank you for sharing your review with pairing bot!"
	case "get-reviews":
		numReviews := 5

		if len(cmdArgs) > 0 {
			numReviews, _ = strconv.Atoi(cmdArgs[0])
		}

		lastN, err := pl.revdb.GetLastN(ctx, numReviews)
		if err != nil {
			log.Printf("Encountered an error when trying to fetch the last %v reviews: %v", numReviews, err)
			response = readErrorMessage
			break
		}

		response = "Here are some reviews of pairing bot:\n"
		for _, rev := range lastN {
			response += "* \"" + rev.content + "\"!\n"
		}
	case "cookie":
		response = cookieClubMessage
	case "help":
		response = helpMessage
	case "version":
		response = pl.version
	default:
		// this won't execute because all input has been sanitized
		// by parseCmd() and all cases are handled explicitly above
	}
	return response, err
}
