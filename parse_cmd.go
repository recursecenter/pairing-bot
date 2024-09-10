package main

import (
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
)

var ErrUnknownCommand = errors.New("unknown command")
var ErrInvalidArguments = errors.New("invalid arguments")

func parseCmd(cmdStr string) (string, []string, error) {
	log.Println("The cmdStr is: ", cmdStr)

	var cmdList = []string{
		"subscribe",
		"unsubscribe",
		"help",
		"schedule",
		"skip",
		"unskip",
		"status",
		"add-review",
		"get-reviews",
		"cookie",
		"version",
	}

	// convert the string to a slice
	// after this, we have a value "cmd" of type []string
	// where cmd[0] is the command and cmd[1:] are any arguments
	space := regexp.MustCompile(`\s+`)
	cmdStr = space.ReplaceAllString(cmdStr, ` `)
	cmdStr = strings.TrimSpace(cmdStr)
	cmdStrLower := strings.ToLower(cmdStr)
	cmd := strings.Split(cmdStrLower, ` `)

	// Big validation logic -- hellooo darkness my old frieeend
	switch {
	// if there's nothing in the command string array
	case len(cmd) == 0:
		// This case is unreachable because strings.Split always returns at
		// least one element.
		return "help", nil, errors.New("the user-issued command was blank")

	// if there's a valid command and if there's no arguments
	case contains(cmdList, cmd[0]) && len(cmd) == 1:
		switch cmd[0] {
		case "schedule":
			return "help", nil, fmt.Errorf("%w: wanted list of days", ErrInvalidArguments)
		case "skip", "unskip":
			return "help", nil, fmt.Errorf(`%w: wanted "tomorrow"`, ErrInvalidArguments)
		case "add-review":
			return "help", nil, fmt.Errorf(`%w: wanted review content`, ErrInvalidArguments)
		}
		return cmd[0], nil, nil

	// if there's a valid command and there's some arguments
	case contains(cmdList, cmd[0]) && len(cmd) > 1:
		switch {
		case cmd[0] == "subscribe" || cmd[0] == "unsubscribe" || cmd[0] == "help" || cmd[0] == "cookie" || cmd[0] == "status":
			return "help", nil, fmt.Errorf("%w: wanted no arguments", ErrInvalidArguments)
		case cmd[0] == "skip" && (len(cmd) != 2 || cmd[1] != "tomorrow"):
			return "help", nil, fmt.Errorf(`%w: wanted "tomorrow"`, ErrInvalidArguments)
		case cmd[0] == "unskip" && (len(cmd) != 2 || cmd[1] != "tomorrow"):
			return "help", nil, fmt.Errorf(`%w: wanted "tomorrow"`, ErrInvalidArguments)
		case cmd[0] == "get-reviews":
			if len(cmd) > 1 {
				if n, err := strconv.Atoi(cmd[1]); err != nil || len(cmd) > 2 || n < 0 {
					return "help", nil, fmt.Errorf(`%w: wanted a positive integer`, ErrInvalidArguments)
				}
			}
			return "get-reviews", cmd[1:], nil
		case cmd[0] == "add-review":
			//We manually split the input cmdStr here since the above code converts it to lower case
			//and we want to presever the user's original formatting/casing
			reviewArgs := strings.SplitN(cmdStr, " ", 2)
			return "add-review", []string{reviewArgs[1]}, nil
		case cmd[0] == "schedule":
			var userSchedule []string

			for _, day := range cmd[1:] {
				fullDayName, err := parseDay(day)
				if err != nil {
					return "help", nil, fmt.Errorf("%w: %w", ErrInvalidArguments, err)
				}

				userSchedule = append(userSchedule, fullDayName)
			}

			return "schedule", userSchedule, nil
		case cmd[0] == "version":
			return "version", nil, nil
		default:
			return cmd[0], cmd[1:], nil
		}

	// if there's not a valid command
	default:
		return "help", nil, fmt.Errorf("%w: %q", ErrUnknownCommand, cmd[0])
	}
}

func contains[S ~[]E, E comparable](list S, element E) bool {
	for _, v := range list {
		if v == element {
			return true
		}
	}
	return false
}

var ErrUnknownDay = errors.New("unknown day abbreviation")

// parseDay expands day name abbreviations into their canonical form.
func parseDay(word string) (string, error) {
	switch strings.ToLower(word) {
	case "mon", "monday":
		return "monday", nil

	case "tu", "tue", "tuesday":
		return "tuesday", nil

	case "wed", "wednesday":
		return "wednesday", nil

	case "th", "thu", "thurs", "thursday":
		return "thursday", nil

	case "fri", "friday":
		return "friday", nil

	case "sat", "saturday":
		return "saturday", nil

	case "sun", "sunday":
		return "sunday", nil

	default:
		return "", fmt.Errorf("%w: %q", ErrUnknownDay, word)
	}
}
