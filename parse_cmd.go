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

	// convert the string to a slice
	// after this, we have a value "cmd" of type []string
	// where cmd[0] is the command and cmd[1:] are any arguments
	space := regexp.MustCompile(`\s+`)
	cmdStr = space.ReplaceAllString(cmdStr, ` `)
	cmdStr = strings.TrimSpace(cmdStr)
	cmdStrLower := strings.ToLower(cmdStr)
	cmd := strings.Split(cmdStrLower, ` `)

	switch name := cmd[0]; name {
	case "subscribe", "unsubscribe", "help", "status", "cookie":
		if len(cmd) > 1 {
			return "help", nil, fmt.Errorf("%w: wanted no arguments", ErrInvalidArguments)
		}
		return name, nil, nil

	case "version":
		// Ignore any extra arguments.
		return name, nil, nil

	case "add-review":
		if len(cmd) == 1 {
			return "help", nil, fmt.Errorf(`%w: wanted review content`, ErrInvalidArguments)
		}

		//We manually split the input cmdStr here since the above code converts it to lower case
		//and we want to presever the user's original formatting/casing
		reviewArgs := strings.SplitN(cmdStr, " ", 2)
		return name, []string{reviewArgs[1]}, nil

	case "get-reviews":
		switch len(cmd) {
		case 1:
			return name, nil, nil
		case 2:
			n, err := strconv.Atoi(cmd[1])
			if err != nil || n < 0 {
				return "help", nil, fmt.Errorf(`%w: wanted a positive integer`, ErrInvalidArguments)
			}
			return name, cmd[1:], nil
		default:
			return "help", nil, fmt.Errorf(`%w: wanted a positive integer`, ErrInvalidArguments)
		}

	case "schedule":
		if len(cmd) == 1 {
			return "help", nil, fmt.Errorf("%w: wanted list of days", ErrInvalidArguments)
		}

		var userSchedule []string

		for _, day := range cmd[1:] {
			fullDayName, err := parseDay(day)
			if err != nil {
				return "help", nil, fmt.Errorf("%w: %w", ErrInvalidArguments, err)
			}

			userSchedule = append(userSchedule, fullDayName)
		}

		return "schedule", userSchedule, nil

	case "skip", "unskip":
		// TODO(#49): Allow (un)skipping days other than tomorrow
		when := "tomorrow"

		if len(cmd) != 2 {
			return "help", nil, fmt.Errorf(`%w: wanted "tomorrow"`, ErrInvalidArguments)
		}

		if cmd[1] != when {
			return "help", nil, fmt.Errorf(`%w: wanted "tomorrow"`, ErrInvalidArguments)
		}
		return name, []string{when}, nil

	default:
		return "help", nil, fmt.Errorf("%w: %q", ErrUnknownCommand, cmd[0])
	}
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
