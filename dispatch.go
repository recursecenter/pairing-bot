package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

const helpMessage string = "**How to use Pairing Bot:**\n" +
	"* `subscribe` to start getting matched with other Pairing Bot users for pair programming\n" +
	"* `schedule mon wed friday` to set your weekly pairing schedule\n  * In this example, I've been set to find pairing partners for you on every Monday, Wednesday, and Friday\n  * You can schedule pairing for any combination of days in the week\n" +
	"* `skip tomorrow` to skip pairing tomorrow\n  * This is valid until matches go out at 04:00 UTC\n" +
	"* `unskip tomorrow` to undo skipping tomorrow\n" +
	"* `status` to show your current schedule, skip status, and name\n" +
	"* `add-review {review_content}` to share a publicly viewable review about Pairing Bot\n" +
	"* `get-reviews` to get recent reviews of Pairing Bot\n  * You can specify the number of reviews to view by specifying `get reviews {num_reviews}`\n" +
	"* `cookie` only use this command if you like :cookie::cookie::cookie:\n" +
	"* `unsubscribe` to stop getting matched entirely\n\n" +
	"If you've found a bug, please [submit an issue on github](https://github.com/stillgreenmoss/pairing-bot/issues)!"
const subscribeMessage string = "Yay! You're now subscribed to Pairing Bot!\nCurrently, I'm set to find pair programming partners for you on **Mondays**, **Tuesdays**, **Wednesdays**, **Thursdays**, and **Fridays**.\nYou can customize your schedule any time with `schedule` :)"
const unsubscribeMessage string = "You're unsubscribed!\nI won't find pairing partners for you unless you `subscribe`.\n\nBe well :)"
const notSubscribedMessage string = "You're not subscribed to Pairing Bot <3"

var writeErrorMessage = fmt.Sprintf("Something went sideways while writing to the database. You should probably ping %v", owner)
var readErrorMessage = fmt.Sprintf("Something went sideways while reading from the database. You should probably ping %v", owner)

func dispatch(ctx context.Context, pl *PairingLogic, cmd string, cmdArgs []string, userID int64, userEmail string, userName string) (string, error) {
	var response string
	var err error

	rec, err := pl.rdb.GetByUserID(ctx, userID, userEmail, userName)
	if err != nil {
		response = readErrorMessage
		return response, err
	}

	isSubscribed := rec.isSubscribed

	// here's the actual actions. command input from
	// the user input has already been sanitized, so we can
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
		rec.schedule = newSchedule

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

		accessToken, err := pl.adb.GetKey(ctx, "rc-accesstoken", "key")
		if err != nil {
			log.Printf("Something weird happened trying to read the RC API access token from the database: %s", err)
		}

		rec.currentlyAtRC = pl.rcapi.userIsCurrentlyAtRC(accessToken, userID)

		if err = pl.rdb.Set(ctx, userID, rec); err != nil {
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

		rec.isSkippingTomorrow = true

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
		rec.isSkippingTomorrow = false

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
		whoami := rec.name

		// get skip status and prepare to write a sentence with it
		var skipStr string
		if rec.isSkippingTomorrow {
			skipStr = " "
		} else {
			skipStr = " not "
		}

		// make a sorted list of their schedule
		var schedule []string
		for _, day := range daysList {
			// this line is a little wild, sorry. it looks so weird because we
			// have to do type assertion on both interface types
			if rec.schedule[strings.ToLower(day)].(bool) {
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
		response = getCookieClubMessage()
	case "help":
		response = helpMessage
	default:
		// this won't execute because all input has been sanitized
		// by parseCmd() and all cases are handled explicitly above
	}
	return response, err
}

func getCookieClubMessage() string {
	message := ""

	message += getCookieEmojiArt() + "\n"
	message += getClubEmojiArt()
	message +=
		"\n **Welcome to Cookie Consumption Club**" +
			"\n Home of the last cookie recipe you will ever need!" +
			"\n :cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie:" +
			"\n```spoiler [Super Thick Chocolate Chip Cookies by Stella Parks](https://www.seriouseats.com/super-thick-chocolate-chip-cookie-recipe)" +
			"\n :cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie:" +
			"\n```spoiler Ingredients" +
			"\n * 4 ounces unsalted American butter (about 1/2 cup; 113g), softened to about 65°F (18°C)" +
			"\n * 4 ounces light brown sugar (about 1/2 cup, firmly packed; 113g)" +
			"\n * 3 1/2 ounces white sugar, preferably well toasted (about 1/2 cup; 100g)" +
			"\n * 1/2 ounce vanilla extract (about 1 tablespoon; 15g)" +
			"\n * 2 teaspoons (8g) Diamond Crystal kosher salt; for table salt, use about half as much by volume or the same weight (plus more for sprinkling, if desired)" +
			"\n * 1 3/4 teaspoons baking powder" +
			"\n * 1 teaspoon baking soda" +
			"\n * Pinch of grated nutmeg" +
			"\n * 2 large eggs (about 3 1/2 ounces; 100g), straight from the fridge" +
			"\n * 10 ounces all-purpose flour (about 2 1/4 cups, spooned; 283g), such as Gold Medal" +
			"\n * 15 ounces assorted chocolate chips (about 2 1/2 cups; 425g), not chopped chocolate" +
			"\n * 8 1/2 ounces raw walnut pieces or lightly toasted pecan pieces (shy 1 3/4 cups; 240g)" +
			"\n```" +
			"\n :cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie::cookie:" +
			"\n```spoiler Directions" +
			"\n 1. To Prepare the Dough: Combine butter, light brown sugar, white sugar, vanilla extract, salt, baking powder, baking soda, and nutmeg in the bowl of a stand mixer fitted with a paddle attachment." +
			"\n 1. Mix on low to moisten, then increase speed to medium and beat until soft, fluffy, and pale, about 8 minutes; halfway through, pause to scrape bowl and beater with a flexible spatula. With mixer running, add eggs one at a time, letting each incorporate fully before adding the next. Reduce speed to low, then add the flour all at once. When flour is incorporated, add chocolate chips and nuts and keep mixing until dough is homogeneous." +
			"\n 1. Divide dough into 8 equal portions (about 6 ounces/170g each) and round each into a smooth ball. Wrap in plastic and refrigerate at least 12 hours before baking; if well protected from air, the dough can be kept in the fridge up to 1 week (see Make Ahead and Storage)." +
			"\n 1. To Bake: Adjust oven rack to middle position and preheat to 350°F (180°C). Line an aluminum half-sheet pan with parchment paper. When the oven comes to temperature, arrange up to 4 portions of cold dough on prepared pan, leaving ample space between them to account for spread. If you like, sprinkle with additional salt to taste." +
			"\n 1. Bake until cookies are puffed and lightly brown, about 22 minutes, or to an internal temperature of between 175 and 185°F (79 and 85°C). The ideal temperature will vary from person to person; future rounds can be baked more or less to achieve desired consistency." +
			"\n 1. Cool cookies directly on baking sheet until no warmer than 100°F (38°C) before serving. Enjoy warm, or within 12 hours; these cookies taste best when freshly baked" +
			"\n```"

	return message
}

func getCookieEmojiArt() string {
	return ":black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square:\n" +
		":black_large_square::black_large_square::cookie::cookie::cookie::black_large_square::black_large_square::cookie::cookie::black_large_square::black_large_square::black_large_square::cookie::cookie::black_large_square::black_large_square::cookie::black_large_square::black_large_square::cookie::black_large_square::cookie::cookie::cookie::black_large_square::cookie::cookie::cookie::cookie::black_large_square:\n" +
		":black_large_square::cookie::black_large_square::black_large_square::black_large_square::black_large_square::cookie::black_large_square::black_large_square::cookie::black_large_square::cookie::black_large_square::black_large_square::cookie::black_large_square::cookie::black_large_square::cookie::black_large_square::black_large_square::black_large_square::cookie::black_large_square::black_large_square::cookie::black_large_square::black_large_square::black_large_square::black_large_square:\n" +
		":black_large_square::cookie::black_large_square::black_large_square::black_large_square::black_large_square::cookie::black_large_square::black_large_square::cookie::black_large_square::cookie::black_large_square::black_large_square::cookie::black_large_square::cookie::cookie::black_large_square::black_large_square::black_large_square::black_large_square::cookie::black_large_square::black_large_square::cookie::cookie::cookie::black_large_square::black_large_square:\n" +
		":black_large_square::cookie::black_large_square::black_large_square::black_large_square::black_large_square::cookie::black_large_square::black_large_square::cookie::black_large_square::cookie::black_large_square::black_large_square::cookie::black_large_square::cookie::black_large_square::cookie::black_large_square::black_large_square::black_large_square::cookie::black_large_square::black_large_square::cookie::black_large_square::black_large_square::black_large_square::black_large_square:\n" +
		":black_large_square::black_large_square::cookie::cookie::cookie::black_large_square::black_large_square::cookie::cookie::black_large_square::black_large_square::black_large_square::cookie::cookie::black_large_square::black_large_square::cookie::black_large_square::black_large_square::cookie::black_large_square::cookie::cookie::cookie::black_large_square::cookie::cookie::cookie::cookie::black_large_square:\n" +
		":black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square:	"

	// return `
	// :black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square:
	// :black_large_square::black_large_square::cookie::cookie::cookie::black_large_square::black_large_square::cookie::cookie::black_large_square::black_large_square::black_large_square::cookie::cookie::black_large_square::black_large_square::cookie::black_large_square::black_large_square::cookie::black_large_square::cookie::cookie::cookie::black_large_square::cookie::cookie::cookie::cookie::black_large_square:
	// :black_large_square::cookie::black_large_square::black_large_square::black_large_square::black_large_square::cookie::black_large_square::black_large_square::cookie::black_large_square::cookie::black_large_square::black_large_square::cookie::black_large_square::cookie::black_large_square::cookie::black_large_square::black_large_square::black_large_square::cookie::black_large_square::black_large_square::cookie::black_large_square::black_large_square::black_large_square::black_large_square:
	// :black_large_square::cookie::black_large_square::black_large_square::black_large_square::black_large_square::cookie::black_large_square::black_large_square::cookie::black_large_square::cookie::black_large_square::black_large_square::cookie::black_large_square::cookie::cookie::black_large_square::black_large_square::black_large_square::black_large_square::cookie::black_large_square::black_large_square::cookie::cookie::cookie::black_large_square::black_large_square:
	// :black_large_square::cookie::black_large_square::black_large_square::black_large_square::black_large_square::cookie::black_large_square::black_large_square::cookie::black_large_square::cookie::black_large_square::black_large_square::cookie::black_large_square::cookie::black_large_square::cookie::black_large_square::black_large_square::black_large_square::cookie::black_large_square::black_large_square::cookie::black_large_square::black_large_square::black_large_square::black_large_square:
	// :black_large_square::black_large_square::cookie::cookie::cookie::black_large_square::black_large_square::cookie::cookie::black_large_square::black_large_square::black_large_square::cookie::cookie::black_large_square::black_large_square::cookie::black_large_square::black_large_square::cookie::black_large_square::cookie::cookie::cookie::black_large_square::cookie::cookie::cookie::cookie::black_large_square:
	// :black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square:
	// `
}

func getClubEmojiArt() string {
	return ":black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square:\n" +
		":black_large_square::black_large_square::cookie::cookie::cookie::black_large_square::cookie::black_large_square::black_large_square::black_large_square::cookie::black_large_square::black_large_square::black_large_square::cookie::black_large_square::cookie::cookie::cookie::cookie::black_large_square:\n" +
		":black_large_square::cookie::black_large_square::black_large_square::black_large_square::black_large_square::cookie::black_large_square::black_large_square::black_large_square::cookie::black_large_square::black_large_square::black_large_square::cookie::black_large_square::cookie::black_large_square::black_large_square::cookie::black_large_square:\n" +
		":black_large_square::cookie::black_large_square::black_large_square::black_large_square::black_large_square::cookie::black_large_square::black_large_square::black_large_square::cookie::black_large_square::black_large_square::black_large_square::cookie::black_large_square::cookie::cookie::cookie::black_large_square::black_large_square:\n" +
		":black_large_square::cookie::black_large_square::black_large_square::black_large_square::black_large_square::cookie::black_large_square::black_large_square::black_large_square::cookie::black_large_square::black_large_square::black_large_square::cookie::black_large_square::cookie::black_large_square::black_large_square::cookie::black_large_square:\n" +
		":black_large_square::black_large_square::cookie::cookie::cookie::black_large_square::cookie::cookie::cookie::black_large_square::black_large_square::cookie::cookie::cookie::black_large_square::black_large_square::cookie::cookie::cookie::cookie::black_large_square:\n" +
		":black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square:\n"

	// return `
	// :black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square:
	// :black_large_square::black_large_square::cookie::cookie::cookie::black_large_square::cookie::black_large_square::black_large_square::black_large_square::cookie::black_large_square::black_large_square::black_large_square::cookie::black_large_square::cookie::cookie::cookie::cookie::black_large_square:
	// :black_large_square::cookie::black_large_square::black_large_square::black_large_square::black_large_square::cookie::black_large_square::black_large_square::black_large_square::cookie::black_large_square::black_large_square::black_large_square::cookie::black_large_square::cookie::black_large_square::black_large_square::cookie::black_large_square:
	// :black_large_square::cookie::black_large_square::black_large_square::black_large_square::black_large_square::cookie::black_large_square::black_large_square::black_large_square::cookie::black_large_square::black_large_square::black_large_square::cookie::black_large_square::cookie::cookie::cookie::black_large_square::black_large_square:
	// :black_large_square::cookie::black_large_square::black_large_square::black_large_square::black_large_square::cookie::black_large_square::black_large_square::black_large_square::cookie::black_large_square::black_large_square::black_large_square::cookie::black_large_square::cookie::black_large_square::black_large_square::cookie::black_large_square:
	// :black_large_square::black_large_square::cookie::cookie::cookie::black_large_square::cookie::cookie::cookie::black_large_square::black_large_square::cookie::cookie::cookie::black_large_square::black_large_square::cookie::cookie::cookie::cookie::black_large_square:
	// :black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square::black_large_square:
	// `
}
