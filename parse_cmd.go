package main

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
)

var ErrUnknownCommand = errors.New("unknown command")
var ErrInvalidArguments = errors.New("invalid arguments")

func parseCmd(cmdStr string) (string, []string, error) {
	cmdStr = strings.TrimSpace(cmdStr)

	log.Println("The cmdStr is: ", cmdStr)

	name, rest, _ := strings.Cut(cmdStr, " ")
	name = strings.ToLower(name)
	rest = strings.TrimSpace(rest)

	switch name {
	case "subscribe", "unsubscribe", "help", "status", "cookie":
		if len(rest) > 0 {
			return "help", nil, fmt.Errorf("%w: wanted no arguments", ErrInvalidArguments)
		}
		return name, nil, nil

	case "version":
		// Ignore any extra arguments.
		return name, nil, nil

	case "add-review":
		if rest == "" {
			return "help", nil, fmt.Errorf(`%w: wanted review content`, ErrInvalidArguments)
		}
		return name, []string{rest}, nil

	case "get-reviews":
		args := strings.Fields(rest)
		switch len(args) {
		case 0:
			return name, nil, nil
		case 1:
			n, err := strconv.Atoi(args[0])
			if err != nil || n < 0 {
				return "help", nil, fmt.Errorf(`%w: wanted a positive integer`, ErrInvalidArguments)
			}
			return name, args, nil
		default:
			return "help", nil, fmt.Errorf(`%w: wanted a positive integer`, ErrInvalidArguments)
		}

	case "schedule":
		args := strings.Fields(rest)
		if len(args) == 0 {
			return "help", nil, fmt.Errorf("%w: wanted list of days", ErrInvalidArguments)
		}

		var userSchedule []string

		for _, day := range args {
			fullDayName, err := parseDay(day)
			if err != nil {
				return "help", nil, fmt.Errorf("%w: %w", ErrInvalidArguments, err)
			}

			userSchedule = append(userSchedule, fullDayName)
		}

		return "schedule", userSchedule, nil

	case "skip", "unskip":
		// TODO(#49): Allow (un)skipping days other than tomorrow
		if rest != "tomorrow" {
			return "help", nil, fmt.Errorf(`%w: wanted "tomorrow"`, ErrInvalidArguments)
		}
		return name, []string{"tomorrow"}, nil
	case "thank", "thanks":
		return "thanks", nil, nil
	default:
		return "help", nil, fmt.Errorf("%w: %q", ErrUnknownCommand, name)
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
