package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/recursecenter/pairing-bot/zulip"
)

const owner string = `@_**Maren Beam (SP2'19)**`
const oddOneOutMessage string = "OK this is awkward.\nThere were an odd number of people in the match-set today, which means that one person couldn't get paired. Unfortunately, it was you -- I'm really sorry :(\nI promise it's not personal, it was very much random. Hopefully this doesn't happen again too soon. Enjoy your day! <3"
const matchedMessage = "Hi you two! You've been matched for pairing :)\n\nHave fun!\n\nNote: In an effort to reduce the frequency of no-show partners, I'll soon start automatically unsubscribing users that I haven't heard from in a while. Please message me back so I know you're an active user (and messages in this chat count!) :heart:"
const offboardedMessage = "Hi! You've been unsubscribed from Pairing Bot.\n\nThis happens at the end of every batch, and everyone is offboarded even if they're still in batch. If you'd like to re-subscribe, just send me a message that says `subscribe`.\n\nBe well! :)"
const introMessage = "Hi! I'm Pairing Bot (she/her)!\n\nSend me a PM that says `subscribe` to get started :smiley:\n\n:pear::robot:\n:octopus::octopus:"
const autoUnsubscribeMessage = ("Hi! I've noticed that it's been a few pairings since I last heard from you. To make sure I only pair active users together, I've automatically unsubscribed you from pairing.\n\nIt's okay though! Send me another `subscribe` message to join back in whenever you like (even now!) :heart:\n\nIf you *are* active and I unsubscribed you anyway, I'm sorry! Please re-subscribe! And then request a non-automated apology from the maintainers :robot:")

// MAX_OPEN_PAIRINGS is the threshold for considering a user to be "inactive".
// After this many unanswered pairings, the user is automatically unsubscribed.
const MAX_OPEN_PAIRINGS = 3

var maintenanceMode = false

// this is the "id" field from zulip, and is a permanent user ID that's not secret
// Pairing Bot's owner can add their ID here for testing. ctrl+f "ownerID" to see where it's used
const ownerID = 215391

type PairingLogic struct {
	rdb   RecurserDB
	adb   APIAuthDB
	pdb   PairingsDB
	revdb ReviewDB
	rcapi RecurseAPI

	zulip   *zulip.Client
	version string
}

func (pl *PairingLogic) handle(w http.ResponseWriter, r *http.Request) {
	var err error

	responder := json.NewEncoder(w)

	// check and authorize the incoming request
	// observation: we only validate requests for /webhooks, i.e. user input through zulip

	ctx := r.Context()

	log.Println("Handling a new Zulip request")

	botAuth, err := pl.adb.GetKey(ctx, "botauth", "token")
	if err != nil {
		log.Println("Something weird happened trying to read the auth token from the database")
	}

	hook, err := zulip.ParseWebhook(r.Body, botAuth)
	if err != nil {
		log.Println(err)
		http.NotFound(w, r) // TODO(@jdkaplan): 401 Unauthorized if token mismatch?
		return
	}

	// Respond to all public messages with an introduction. Don't process any
	// commands in open streams/channels.
	if hook.Trigger != "direct_message" {
		if err := responder.Encode(zulip.Reply(introMessage)); err != nil {
			log.Println(err)
		}
		return
	}

	// Reset the open pairings count on user activity.
	if err := pl.resetPairingCount(ctx, hook.Message); err != nil {
		log.Printf("Could not reset openPairings count for user (%d): %s", hook.Message.SenderID, err)
	}

	// Don't respond to commands sent in pair-making group DMs. We can
	// distinguish these by checking whether there are exactly two participants
	// (Pairing Bot + 1).
	if len(hook.Message.DisplayRecipient.Users) != 2 {
		if err := responder.Encode(zulip.NoResponse()); err != nil {
			log.Println(err)
		}
		return
	}

	// for testing only
	// this responds with a maintenance message and quits if the request is coming from anyone other than the owner
	if maintenanceMode && hook.Message.SenderID != ownerID {
		if err = responder.Encode(zulip.Reply(`pairing bot is down for maintenance`)); err != nil {
			log.Println(err)
		}
		return
	}

	log.Printf("The user: %s (%d) issued the following request to Pairing Bot: %s", hook.Message.SenderFullName, hook.Message.SenderID, hook.Data)

	// you *should* be able to throw any string at this thing and get back a valid command for dispatch()
	// if there are no command arguments, cmdArgs will be nil
	cmd, cmdArgs, err := parseCmd(hook.Data)
	if err != nil {
		log.Println(err)
		return
	}

	// the tofu and potatoes right here y'all
	response, err := dispatch(ctx, pl, cmd, cmdArgs, hook.Message.SenderID, hook.Message.SenderEmail, hook.Message.SenderFullName)
	if err != nil {
		log.Println(err)
		return
	}

	if err = responder.Encode(zulip.Reply(response)); err != nil {
		log.Println(err)
		return
	}
}

// "match" makes matches for pairing, and messages those people to notify them of their match
// it runs once per day (it's triggered with app engine's cron service)
func (pl *PairingLogic) match(w http.ResponseWriter, r *http.Request) {
	// Check that the request is originating from within app engine
	// https://cloud.google.com/appengine/docs/flexible/go/scheduling-jobs-with-cron-yaml#validating_cron_requests
	if r.Header.Get("X-Appengine-Cron") != "true" {
		http.NotFound(w, r)
		return
	}

	ctx := r.Context()

	recursersList, err := pl.rdb.ListPairingTomorrow(ctx)
	log.Println(recursersList)
	if err != nil {
		log.Printf("Could not get list of recursers from DB: %s\n", err)
	}

	skippersList, err := pl.rdb.ListSkippingTomorrow(ctx)
	if err != nil {
		log.Printf("Could not get list of skippers from DB: %s\n", err)
	}

	// get everyone who was set to skip today and set them back to isSkippingTomorrow = false
	for _, skipper := range skippersList {
		err := pl.rdb.UnsetSkippingTomorrow(ctx, skipper)
		if err != nil {
			log.Printf("Could not unset skipping for recurser %v: %s\n", skipper.id, err)
		}
	}

	// Reproducible randomness:
	// - Get and log a random seed
	// - Run the shuffle using a source derived from that seed
	// so we can re-run the shuffle later, if needed.
	// In dev, you should be able to set the seed below to get the same shuffle.
	seed := rand.Int63()
	log.Printf("Shuffling %d Recursers using random seed: %d", len(recursersList), seed)
	randSrc := rand.NewSource(seed)
	// shuffle our recursers. This will not error if the list is empty
	rand.New(randSrc).Shuffle(len(recursersList), func(i, j int) { recursersList[i], recursersList[j] = recursersList[j], recursersList[i] })

	// if for some reason there's no matches today, we're done
	if len(recursersList) == 0 {
		log.Println("No one was signed up to pair today -- so there were no matches")
		return
	}

	// Remove anyone who has been inactive for too many pairings.
	active, inactive := groupByActivity(recursersList)

	// message the peeps!

	// if there's an odd number today, message the last person in the list
	// and tell them they don't get a match today, then knock them off the list
	if len(active)%2 != 0 {
		recurser := active[len(active)-1]
		active = active[:len(active)-1]
		log.Printf("%s was the odd-one-out today", recurser.name)

		err := pl.zulip.SendUserMessage(ctx, []int64{recurser.id}, oddOneOutMessage)
		if err != nil {
			log.Printf("Error when trying to send oddOneOut message to %s: %s\n", recurser.name, err)
		}
	}

	for i := 0; i < len(active); i += 2 {
		rc1 := &active[i]
		rc2 := &active[i+1]
		ids := []int64{rc1.id, rc2.id}

		err := pl.zulip.SendUserMessage(ctx, ids, matchedMessage)
		if err != nil {
			log.Printf("Error when trying to send matchedMessage to %s and %s: %s\n", rc1.name, rc2.name, err)
			continue
		}
		log.Println(rc1.name, "was", "matched", "with", rc2.name)

		for _, rc := range []*Recurser{rc1, rc2} {
			if err := pl.incrementPairingCount(ctx, rc); err != nil {
				log.Printf("Error incrementing openPairings for user (%d): %s", rc1.id, err)
			}
		}
	}

	numRecursersPairedUp := len(active)

	log.Printf("Pairing Bot paired up %d recursers today", numRecursersPairedUp)

	numPairings := numRecursersPairedUp / 2

	timestamp := time.Now().Unix()
	if err := pl.pdb.SetNumPairings(ctx, int(timestamp), numPairings); err != nil {
		log.Printf("Failed to record today's pairings: %s", err)
	}

	for _, r := range inactive {
		if err := pl.rdb.Delete(ctx, r.id); err != nil {
			log.Printf("Could not unsubscribe user (%d): %s", r.id, err)
		}

		if err := pl.zulip.SendUserMessage(ctx, []int64{r.id}, autoUnsubscribeMessage); err != nil {
			log.Printf("Could not send auto-unsubscribe message to user (%d): %s", r.id, err)
		}
	}

	log.Printf("Pairing Bot auto-unsubscribed %d inactive recursers today", len(inactive))
}

// Unsubscribe people from Pairing Bot when their batch is over. They're always welcome to re-subscribe manually!
func (pl *PairingLogic) endofbatch(w http.ResponseWriter, r *http.Request) {
	// Check that the request is originating from within app engine
	// https://cloud.google.com/appengine/docs/flexible/go/scheduling-jobs-with-cron-yaml#validating_cron_requests
	if r.Header.Get("X-Appengine-Cron") != "true" {
		http.NotFound(w, r)
		return
	}

	// getting all the recursers
	ctx := r.Context()
	recursersList, err := pl.rdb.GetAllUsers(ctx)
	if err != nil {
		log.Println("Could not get list of recursers from DB: ", err)
	}

	accessToken, err := pl.adb.GetKey(ctx, "rc-accesstoken", "key")
	if err != nil {
		log.Println("Something weird happened trying to read the RC API access token from the database: ", err)
	}

	idsOfPeopleAtRc, err := pl.rcapi.getCurrentlyActiveZulipIds(accessToken)
	if err != nil {
		log.Println("Encountered error while getting currently-active Recursers: ", err)
		// TODO: https://github.com/recursecenter/pairing-bot/issues/61: Alert here!
		// Using a FATAL here so it gets called out in the logs.
		log.Fatal("Aborting end-of-batch processing!")
		return
	}

	for i := 0; i < len(recursersList); i++ {

		recurser := recursersList[i]

		isAtRCThisWeek := contains(idsOfPeopleAtRc, recurser.id)
		wasAtRCLastWeek := recursersList[i].currentlyAtRC

		log.Printf("User: %s was at RC last week: %t and is at RC this week: %t", recurser.name, wasAtRCLastWeek, isAtRCThisWeek)

		recurser.currentlyAtRC = isAtRCThisWeek

		if err = pl.rdb.Set(ctx, recurser.id, recurser); err != nil {
			log.Printf("Error encountered while update currentlyAtRC status for user: %s (ID %d)", recurser.name, recurser.id)
		}

		// If they were at RC last week but not this week then we assume they have graduated or otherwise left RC
		// In that case we remove them from pairing bot so that inactive people do not get matched
		// If people who have left RC still want to use pairing bot, we give them the option to resubscribe
		if wasAtRCLastWeek && !isAtRCThisWeek {
			var message string

			err = pl.rdb.Delete(ctx, recurser.id)
			if err != nil {
				log.Println(err)
				message = fmt.Sprintf("Uh oh, I was trying to offboard you since it's the end of batch, but something went wrong. Consider messaging %v to let them know this happened.", owner)
			} else {
				log.Printf("This user has been unsubscribed from pairing bot: %s (ID: %d)", recurser.name, recurser.id)

				message = offboardedMessage
			}

			err := pl.zulip.SendUserMessage(ctx, []int64{recurser.id}, message)
			if err != nil {
				log.Printf("Error when trying to send offboarding message to %s (ID %d): %s", recurser.name, recurser.id, err)
			}
		}
	}
}

func (pl *PairingLogic) checkin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	numPairings, err := pl.pdb.GetTotalPairingsDuringLastWeek(ctx)

	if err != nil {
		log.Println("Unable to get the total number of pairings during the last week: : ", err)
	}

	recursersList, err := pl.rdb.GetAllUsers(ctx)
	if err != nil {
		log.Printf("Could not get list of recursers from DB: %s\n", err)
	}

	review, err := pl.revdb.GetRandom(ctx)
	if err != nil {
		log.Println("Could not get a random review from DB: ", err)
	}

	checkinMessage := getCheckinMessage(numPairings, len(recursersList), review.content)

	if err := pl.zulip.PostToTopic(ctx, "checkins", "Pairing Bot", checkinMessage); err != nil {
		log.Printf("Error when trying to submit Pairing Bot checkins stream message: %s\n", err)
	}
}

func getCheckinMessage(numPairings int, numRecursers int, review string) string {
	today := time.Now()
	todayFormatted := today.Format("January 2, 2006")

	message :=
		"```Bash\n" +
			"=> Initializing the Pairing Bot process\n" +
			"######################################################################## 100%%\n" +
			"=> Loading Pairing Bot Usage Statistics\n" +
			"######################################################################## 100%%\n" +
			"=> Teaching Pairing Bot how to boop beep boop as it is a strange loop\n" +
			"######################################################################## 00110001 00110000 00110000 00100101\n\n" +
			"``` \n\n\n" +
			"**%s Checkin**\n\n" +
			"* Current number of Recursers subscribed to Pairing Bot: %d\n\n" +
			"* Number of pairings facilitiated in the last week: %d \n\n" +
			"**Randomly Selected Pairing Bot Review**\n\n" +
			"* %s"

	return fmt.Sprintf(message, todayFormatted, numRecursers, numPairings, review)
}

/*
Sends out a "Welcome to Pairing Bot" message to 397 Bridge during the second week of RC to introduce people to RC.

We don't send this welcome message during the first week since it's a bit overwhelming with all of the orientation meetings
and people haven't had time to think too much about their projects.
*/
func (pl *PairingLogic) welcome(w http.ResponseWriter, r *http.Request) {
	// Check that the request is originating from within app engine
	// https://cloud.google.com/appengine/docs/flexible/go/scheduling-jobs-with-cron-yaml#validating_cron_requests
	if r.Header.Get("X-Appengine-Cron") != "true" {
		http.NotFound(w, r)
		return
	}

	ctx := r.Context()

	accessToken, err := pl.adb.GetKey(ctx, "rc-accesstoken", "key")
	if err != nil {
		log.Printf("Something weird happened trying to read the RC API access token from the database: %s", err)
	}

	if pl.rcapi.isSecondWeekOfBatch(accessToken) {
		if err := pl.zulip.PostToTopic(ctx, "397 Bridge", "🍐🤖", getWelcomeMessage()); err != nil {
			log.Printf("Error when trying to send welcome message about Pairing Bot %s\n", err)
		}
	}
}

func getWelcomeMessage() string {
	today := time.Now()
	todayFormatted := today.Format("01.02.2006")

	message :=
		"```Bash\n" +
			"=> Initializing the Pairing Bot process\n" +
			"######################################################################## 100%%\n" +
			"=> Loading list of people currently at RC\n" +
			"######################################################################## 100%%\n" +
			"=> Teaching Pairing Bot how to beep boop beep\n" +
			"######################################################################## 00110001 00110000 00110000 00100101\n\n" +
			"=> Pairing Bot successfully updated to version %s\n" +
			"``` \n\n\n" +
			"Greetings @*Currently at RC*,\n\n" +
			"My name is Pairing Bot and my mission is to ~~eliminate all~~ help pair people at RC to work on projects.\n\n" +
			"**How To Get Started**\n\n" +
			"* Send me a private message with the word `subscribe` to get started. I will then match you with another pairing bot subscriber each day.\n\n" +
			"* Don't want to pair each day? You can set your schedule with the command `schedule tuesday friday` and I will only match you with people on those days.\n\n" +
			"* You can view a full list of my functions by sending me a PM with the message `help`.\n\n" +
			"* See what other recursers have to say about me by using the `get-reviews` command."

	return fmt.Sprintf(message, todayFormatted)
}

func (pl *PairingLogic) resetPairingCount(ctx context.Context, msg zulip.Message) error {
	rec, err := pl.rdb.GetByUserID(ctx, msg.SenderID, msg.SenderEmail, msg.SenderFullName)
	if err != nil {
		return err
	}

	if rec.isSubscribed {
		rec.openPairings = 0
		return pl.rdb.Set(ctx, rec.id, rec)
	}

	return nil
}

func (pl *PairingLogic) incrementPairingCount(ctx context.Context, rc *Recurser) error {
	rc.openPairings++
	return pl.rdb.Set(ctx, rc.id, *rc)
}

func groupByActivity(recursers []Recurser) (active, inactive []Recurser) {
	for _, r := range recursers {
		if r.openPairings < MAX_OPEN_PAIRINGS {
			active = append(active, r)
		} else {
			inactive = append(inactive, r)
		}
	}
	return active, inactive
}
