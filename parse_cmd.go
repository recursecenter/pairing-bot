package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

type parsingErr struct{ msg string }

func (e parsingErr) Error() string {
	return fmt.Sprintf("Error when parsing command: %s", e.msg)
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
		"status",
		"add-review",
	}

	//This also includes common abbreviations for the days of the week
	var daysList = []string{
		"monday",
		"tuesday",
		"wednesday",
		"thursday",
		"friday",
		"saturday",
		"sunday",
		"mon",
		"tu",
		"tue",
		"tues",
		"wed",
		"th",
		"thu",
		"thur",
		"thurs",
		"fri",
		"sat",
		"sun"}

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
		err = errors.New("the user-issued command was blank")
		return "help", nil, err

	// if there's a valid command and if there's no arguments
	case contains(cmdList, cmd[0]) && len(cmd) == 1:
		if cmd[0] == "schedule" || cmd[0] == "skip" || cmd[0] == "unskip" || cmd[0] == "review" {
			err = &parsingErr{"the user issued a command without args, but it reqired args"}
			return "help", nil, err
		}
		return cmd[0], nil, err

	// if there's a valid command and there's some arguments
	case contains(cmdList, cmd[0]) && len(cmd) > 1:
		switch {
		case cmd[0] == "subscribe" || cmd[0] == "unsubscribe" || cmd[0] == "help" || cmd[0] == "status":
			err = &parsingErr{"the user issued a command with args, but it disallowed args"}
			return "help", nil, err
		case cmd[0] == "skip" && (len(cmd) != 2 || cmd[1] != "tomorrow"):
			err = &parsingErr{"the user issued SKIP with malformed arguments"}
			return "help", nil, err
		case cmd[0] == "unskip" && (len(cmd) != 2 || cmd[1] != "tomorrow"):
			err = &parsingErr{"the user issued UNSKIP with malformed arguments"}
			return "help", nil, err
		case cmd[0] == "add-review":
			//We manually split the input cmdStr here since the above code converts it to lower case
			//and we want to presever the user's original formatting/casing
			reviewArgs := strings.SplitN(cmdStr, " ", 2)
			return "review", []string{reviewArgs[1]}, err
		case cmd[0] == "schedule":
			for _, v := range cmd[1:] {
				if !contains(daysList, v) {
					err = &parsingErr{"the user issued SCHEDULE with malformed arguments"}
					return "help", nil, err
				}
			}
			fallthrough
		default:
			return cmd[0], cmd[1:], err
		}

	// if there's not a valid command
	default:
		err = &parsingErr{"the user-issued command wasn't valid"}
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
