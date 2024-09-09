package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

func (pl *PairingLogic) dispatch(ctx context.Context, cmd string, cmdArgs []string, rec *Recurser) (string, error) {
	// here's the actual actions. command input from
	// the user input has already been sanitized, so we can
	// trust that cmd and cmdArgs only have valid stuff in them
	switch cmd {
	case "schedule":
		return pl.SetSchedule(ctx, rec, cmdArgs)

	case "subscribe":
		return pl.Subscribe(ctx, rec)

	case "unsubscribe":
		return pl.Unsubscribe(ctx, rec)

	case "skip":
		return pl.SkipTomorrow(ctx, rec)

	case "unskip":
		return pl.UnskipTomorrow(ctx, rec)

	case "status":
		return pl.Status(ctx, rec)

	case "add-review":
		content := cmdArgs[0]
		return pl.AddReview(ctx, rec, content)

	case "get-reviews":
		numReviews := 5
		if len(cmdArgs) > 0 {
			numReviews, _ = strconv.Atoi(cmdArgs[0])
		}
		return pl.GetReviews(ctx, numReviews)

	case "cookie":
		return cookieClubMessage, nil

	case "help":
		return helpMessage, nil

	case "version":
		return pl.version, nil

	default:
		// this won't execute because all input has been sanitized
		// by parseCmd() and all cases are handled explicitly above
		return "", nil
	}
}

func (pl *PairingLogic) SetSchedule(ctx context.Context, rec *Recurser, days []string) (string, error) {
	if !rec.IsSubscribed {
		return notSubscribedMessage, nil
	}

	rec.Schedule = newSchedule(days)

	if err := pl.rdb.Set(ctx, rec.ID, rec); err != nil {
		return writeErrorMessage, err
	}
	return "Awesome, your new schedule's been set! You can check it with `status`.", nil
}

func (pl *PairingLogic) Subscribe(ctx context.Context, rec *Recurser) (string, error) {
	if rec.IsSubscribed {
		return "You're already subscribed! Use `schedule` to set your schedule.", nil
	}

	atRC, err := pl.recurse.IsCurrentlyAtRC(ctx, rec.ID)
	if err != nil {
		log.Printf("Could not read currently-at-RC data from RC API: %s", err)
		return readErrorMessage, err
	}

	rec.CurrentlyAtRC = atRC

	if err = pl.rdb.Set(ctx, rec.ID, rec); err != nil {
		log.Printf("Could not update recurser in database: %s", err)
		return writeErrorMessage, err
	}
	return subscribeMessage, nil
}

func (pl *PairingLogic) Unsubscribe(ctx context.Context, rec *Recurser) (string, error) {
	if !rec.IsSubscribed {
		return notSubscribedMessage, nil
	}

	if err := pl.rdb.Delete(ctx, rec.ID); err != nil {
		return writeErrorMessage, err
	}
	return unsubscribeMessage, nil
}

func (pl *PairingLogic) SkipTomorrow(ctx context.Context, rec *Recurser) (string, error) {
	if !rec.IsSubscribed {
		return notSubscribedMessage, nil
	}

	rec.IsSkippingTomorrow = true

	if err := pl.rdb.Set(ctx, rec.ID, rec); err != nil {
		return writeErrorMessage, err
	}
	return `Tomorrow: cancelled. I feel you. **I will not match you** for pairing tomorrow <3`, nil
}

func (pl *PairingLogic) UnskipTomorrow(ctx context.Context, rec *Recurser) (string, error) {
	if !rec.IsSubscribed {
		return notSubscribedMessage, nil
	}
	rec.IsSkippingTomorrow = false

	if err := pl.rdb.Set(ctx, rec.ID, rec); err != nil {
		return writeErrorMessage, err
	}
	return "Tomorrow: uncancelled! Heckin *yes*! **I will match you** for pairing tomorrow :)", nil
}

func (pl *PairingLogic) Status(ctx context.Context, rec *Recurser) (string, error) {
	if !rec.IsSubscribed {
		return notSubscribedMessage, nil
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

	return fmt.Sprintf("* You're %v\n* You're scheduled for pairing on **%v**\n* **You're%vset to skip** pairing tomorrow", whoami, scheduleStr, skipStr), nil
}

func (pl *PairingLogic) AddReview(ctx context.Context, rec *Recurser, content string) (string, error) {
	currentTimestamp := time.Now().Unix()

	err := pl.revdb.Insert(ctx, Review{
		content:   content,
		timestamp: int(currentTimestamp),
		email:     rec.Email,
	})
	if err != nil {
		log.Println("Encountered an error when trying to save a review: ", err)
		return writeErrorMessage, err
	}

	return "Thank you for sharing your review with pairing bot!", nil
}

func (pl *PairingLogic) GetReviews(ctx context.Context, numReviews int) (string, error) {
	lastN, err := pl.revdb.GetLastN(ctx, numReviews)
	if err != nil {
		log.Printf("Encountered an error when trying to fetch the last %v reviews: %v", numReviews, err)
		return readErrorMessage, err
	}

	response := "Here are some reviews of pairing bot:\n"
	for _, rev := range lastN {
		response += "* \"" + rev.content + "\"!\n"
	}
	return response, nil
}
