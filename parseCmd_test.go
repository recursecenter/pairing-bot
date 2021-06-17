package main

import (
	"testing"
)

var daysList = map[string]struct{}{
	"monday":    {},
	"tuesday":   {},
	"wednesday": {},
	"thursday":  {},
	"friday":    {},
	"saturday":  {},
	"sunday":    {},
}

// due to the difficulties around comparing error return values (and because we don't want to compare error messages),
// the struct contains expectErr to indicate whether an error is expected, instead of an actual error value
var tableNoArgs = []struct {
	testName   string
	inputStr   string
	wantedCmd  string
	wantedArgs []string
	expectErr  bool
}{
	{"subscribe_correct_usage", "subscribe", "subscribe", nil, false},
	{"subscribe_wrong_usage", "subscribe mon", "help", nil, true},
	{"unsubscribe_correct_usage", "unsubscribe", "unsubscribe", nil, false},
	{"unsubscribe_wrong_usage", "unsubscribe tuesday", "help", nil, true},
	{"help_correct_usage", "help", "help", nil, false},
	{"help_wrong_usage", "help me", "help", nil, true},
	{"status_correct_usage", "status", "status", nil, false},
	{"status_wrong_usage", "status me", "help", nil, true},
}

func TestParseCmdNoArgs(t *testing.T) {
	for _, tt := range tableNoArgs {
		t.Run(tt.testName, func(t *testing.T) {
			gotCmd, gotArgs, gotErr := parseCmd(tt.inputStr)
			if gotCmd != tt.wantedCmd {
				t.Errorf("got %v, %v\n", gotCmd, gotArgs)
			}

			_, ok := gotErr.(*parsingErr)

			if tt.expectErr && !ok {
				t.Errorf("Expected parsingErr but didn't get one\n")
			} else if !tt.expectErr && ok {
				t.Errorf("Got unexpected parsingError\n")
			}
		})
	}
}

var tableWithArgs = []struct {
	testName   string
	inputStr   string
	wantedCmd  string
	wantedArgs []string
	expectErr  bool
}{
	{"schedule_1_arg", "schedule monday", "schedule", []string{"monday"}, false},
	{"schedule_2_args", "schedule monday friday", "schedule", []string{"monday", "friday"}, false},
	{"schedule_3_args", "schedule monday wednesday friday", "schedule", []string{"monday", "wednesday", "friday"}, false},
	{"schedule_4_args", "schedule monday wednesday friday sunday", "schedule", []string{"monday", "wednesday", "friday", "sunday"}, false},
	{"schedule_weekend_only", "schedule sunday", "schedule", []string{"sunday"}, false},
	{"schedule_wrong_usage", "schedule", "help", nil, true},
	{"skip_correct_usage", "skip tomorrow", "skip", []string{"tomorrow"}, false},
	{"skip_wrong_usage", "skip monday", "help", nil, true},
	{"skip_wrong_usage", "skip whenever", "help", nil, true},
	{"skip_wrong_usage", "skip", "help", nil, true},
	{"unskip_correct_usage", "unskip tomorrow", "unskip", []string{"tomorrow"}, false},
	{"unskip_wrong_usage", "unskip today", "help", nil, true},
	{"unskip_wrong_usage", "unskip friday", "help", nil, true},
	{"unskip_wrong_usage", "unskip", "help", nil, true},
}

func TestParseCmdWithArgs(t *testing.T) {
	for _, tt := range tableWithArgs {
		t.Run(tt.testName, func(t *testing.T) {
			gotCmd, gotArgs, gotErr := parseCmd(tt.inputStr)
			if gotCmd != tt.wantedCmd || len(gotArgs) != len(tt.wantedArgs) {
				t.Errorf("got %v, %v, wanted %v, %v\n", gotCmd, gotArgs, tt.wantedCmd, tt.wantedArgs)
			}

			switch gotCmd {
			case "schedule":
				for i, arg := range gotArgs {
					if _, ok := daysList[arg]; !ok {
						t.Errorf("Wrong argument %v for command %v\n", gotArgs[i], gotCmd)
					}
				}
			case "skip":
				if gotArgs[0] != "tomorrow" {
					t.Errorf("Wrong argument %v for command %v\n", gotArgs[0], gotCmd)
				}
			case "unskip":
				if gotArgs[0] != "tomorrow" {
					t.Errorf("Wrong argument %v for command %v\n", gotArgs[0], gotCmd)
				}
			default:
				if gotCmd != "help" {
					t.Errorf("unknown command %v\n", gotCmd)
				}
			}

			_, ok := gotErr.(*parsingErr)

			if tt.expectErr && !ok {
				t.Errorf("Expected parsingErr but didn't get one\n")
			} else if !tt.expectErr && ok {
				t.Errorf("Got unexpected parsingError\n")
			}
		})
	}
}

var tableMisc = []struct {
	testName   string
	inputStr   string
	wantedCmd  string
	wantedArgs []string
	expectErr  bool
}{
	{"command_is_superstring", "scheduleing monday", "help", nil, true},
	{"command_is_substring", "schedul monday", "help", nil, true},
	{"command_is_undefined", "mooh", "help", nil, true},
	{"command_is_capitalized", "Help", "help", nil, false},
}

func TestParseCmdMisc(t *testing.T) {
	for _, tt := range tableMisc {
		t.Run(tt.testName, func(t *testing.T) {
			gotCmd, gotArgs, gotErr := parseCmd(tt.inputStr)
			if gotCmd != tt.wantedCmd {
				t.Errorf("got %v, %v, wanted %v, %v\n", gotCmd, gotArgs, tt.wantedCmd, tt.wantedArgs)
			}
			_, ok := gotErr.(*parsingErr)

			if tt.expectErr && !ok {
				t.Errorf("Expected parsingErr but didn't get one\n")
			} else if !tt.expectErr && ok {
				t.Errorf("Got unexpected parsingError\n")
			}
		})
	}
}
