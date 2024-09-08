package main

import (
	_ "embed"
	"fmt"
)

//go:embed messages/odd_one_out.md
var oddOneOutMessage string

//go:embed messages/matched.md
var matchedMessage string

//go:embed messages/offboarded.md
var offboardedMessage string

//go:embed messages/intro.md
var introMessage string

//go:embed messages/cookieClub.md
var cookieClubMessage string

//go:embed messages/help.md
var helpMessage string

//go:embed messages/subscribed.md
var subscribeMessage string

//go:embed messages/unsubscribed.md
var unsubscribeMessage string

const notSubscribedMessage string = "You're not subscribed to Pairing Bot <3"

var writeErrorMessage = fmt.Sprintf("Something went sideways while writing to the database. You should probably ping %v", maintainersMention())
var readErrorMessage = fmt.Sprintf("Something went sideways while reading from the database. You should probably ping %v", maintainersMention())
