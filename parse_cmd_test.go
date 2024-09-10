package main

import (
	"testing"

	"github.com/recursecenter/pairing-bot/internal/assert"
)

type parseResult struct {
	Cmd  string
	Args []string
}

var acceptedCommands = map[string]parseResult{
	"subscribe":   {"subscribe", nil},
	"unsubscribe": {"unsubscribe", nil},
	"help":        {"help", nil},
	"status":      {"status", nil},
	"get-reviews": {"get-reviews", nil},
	"cookie":      {"cookie", nil},
	"version":     {"version", nil},

	// This command ignores its arguments.
	"version info": {"version", nil},

	// These commands require exact literal arguments.
	"skip tomorrow":   {"skip", []string{"tomorrow"}},
	"unskip tomorrow": {"unskip", []string{"tomorrow"}},

	// Schedules!
	"schedule monday":         {"schedule", []string{"monday"}},
	"schedule sunday":         {"schedule", []string{"sunday"}},
	"schedule friday tuesday": {"schedule", []string{"friday", "tuesday"}},
	"schedule mon tue wed thu fri sat sun": {
		"schedule",
		[]string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"},
	},

	// BUG(@jdkaplan): Don't squash spaces *inside* the review.
	"add-review  :pear: ing    :robot:": {"add-review", []string{":pear: ing :robot:"}},

	"get-reviews 0":  {"get-reviews", []string{"0"}},
	"get-reviews 1":  {"get-reviews", []string{"1"}},
	"get-reviews 5":  {"get-reviews", []string{"5"}},
	"get-reviews 10": {"get-reviews", []string{"10"}},

	// Commands are case-insensitive.
	"Help":      {"help", nil},
	"hElP":      {"help", nil},
	"sUbScRiBe": {"subscribe", nil},

	// Day names (as keywords) are also case-insensitive
	"schedule MoN WED fRi": {"schedule", []string{"monday", "wednesday", "friday"}},

	// Review content *is* case-sensitive.
	"add-review   I :heart: Pairing Bot!\n": {"add-review", []string{"I :heart: Pairing Bot!"}},
}

func TestParseCmdAccept(t *testing.T) {
	for input, want := range acceptedCommands {
		t.Run(input, func(t *testing.T) {
			cmd, args, err := parseCmd(input)
			if err != nil {
				t.Fatalf("unexpected error: %#+v", err)
			}

			assert.Equal(t, cmd, want.Cmd)
			assert.Equal(t, args, want.Args)
		})
	}
}

var rejectedCommands = map[string]error{
	"": nil,

	// Funnily enough: nil, these *do* give you what you want!
	"help me":       nil,
	"halp":          nil,
	"schedule help": ErrUnknownDay,

	// Unexpected arguments
	"status me": nil,
	"cookie me": nil,

	// Did they really want `schedule`?
	"subscribe tue":   nil,
	"unsubscribe thu": nil,

	// (Un)skipping requires an argument.
	"skip":   nil,
	"unskip": nil,

	// TODO(#49): Allow (un)skipping days other than tomorrow
	"skip friday": nil,
	"unskip next": nil,

	// This is not the way to delete reviews you don't like 😛
	"get-reviews -1":  nil,
	"get-reviews -10": nil,

	// Unknown commands
	"scheduleing monday": nil,
	"schedul monday":     nil,
	"mooh":               nil,
}

func TestParseCmdReject(t *testing.T) {
	for input, want := range rejectedCommands {
		t.Run(input, func(t *testing.T) {
			cmd, args, err := parseCmd(input)

			if want == nil {
				_, _ = assert.ErrorAs[*parsingErr](t, err)
			} else {
				assert.ErrorIs(t, err, want)
			}

			assert.Equal(t, cmd, "help")
			assert.Equal(t, args, nil)
		})
	}
}
