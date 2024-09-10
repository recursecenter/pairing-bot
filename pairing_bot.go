package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/recursecenter/pairing-bot/recurse"
	"github.com/recursecenter/pairing-bot/zulip"
)

var maintenanceMode = false

// maintainers contains the Zulip IDs of the current maintainers.
//
// This is a map instead of a slice to allow for easy membership checks.
var maintainers = map[int64]struct{}{
	215391: {}, // Maren Beam (SP2'19)
	699369: {}, // Charles Eckman (SP2'24)
	720507: {}, // Jeremy Kaplan (S1'24)
}

// isMaintainer returns whether this Zulip ID is in the maintainer set.
func isMaintainer(id int64) bool {
	_, isPresent := maintainers[id]
	return isPresent
}

// maintainersMention returns a Zulip-markdown string that mentions all the
// maintainers.
//
// https://zulip.com/help/format-your-message-using-markdown#mention-a-user-or-group
func maintainersMention() string {
	var tags []string
	for id := range maintainers {
		tags = append(tags, fmt.Sprintf("@_**|%d**", id))
	}
	return strings.Join(tags, ", ")
}

type PairingLogic struct {
	rdb   *FirestoreRecurserDB
	adb   *FirestoreAPIAuthDB
	pdb   PairingsDB
	revdb ReviewDB

	zulip   *zulip.Client
	recurse *recurse.Client
	version string
}

func (pl *PairingLogic) handle(w http.ResponseWriter, r *http.Request) {
	var err error

	responder := json.NewEncoder(w)

	// check and authorize the incoming request
	// observation: we only validate requests for /webhooks, i.e. user input through zulip

	ctx := r.Context()

	log.Println("Handling a new Zulip request")

	botAuth, err := pl.adb.GetToken(ctx, "botauth/token")
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
	// this responds with a maintenance message and quits if the request is coming from anyone other than a maintainer
	if !isMaintainer(hook.Message.SenderID) && maintenanceMode {
		if err = responder.Encode(zulip.Reply(`pairing bot is down for maintenance`)); err != nil {
			log.Println(err)
		}
		return
	}

	log.Printf("The user: %s (%d) issued the following request to Pairing Bot: %s", hook.Message.SenderFullName, hook.Message.SenderID, hook.Data)

	user, err := pl.rdb.GetByUserID(ctx, hook.Message.SenderID, hook.Message.SenderEmail, hook.Message.SenderFullName)
	if err != nil {
		log.Println(err)

		if err = responder.Encode(zulip.Reply(readErrorMessage)); err != nil {
			log.Println(err)
		}
		return
	}

	// you *should* be able to throw any string at this thing and get back a valid command for dispatch()
	// if there are no command arguments, cmdArgs will be nil
	cmd, cmdArgs, err := parseCmd(hook.Data)
	if err != nil {
		log.Println(err)
		// Error cases always correspond to cmd == "help", so it's safe to
		// continue on to dispatch.
	}

	// the tofu and potatoes right here y'all
	response, err := pl.dispatch(ctx, cmd, cmdArgs, user)
	if err != nil {
		log.Println(err)
		// Errors come with non-empty messages sometimes, so continue on.
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
		err := pl.rdb.UnsetSkippingTomorrow(ctx, &skipper)
		if err != nil {
			log.Printf("Could not unset skipping for recurser %v: %s\n", skipper.ID, err)
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

	// message the peeps!

	// if there's an odd number today, message the last person in the list
	// and tell them they don't get a match today, then knock them off the list
	if len(recursersList)%2 != 0 {
		recurser := recursersList[len(recursersList)-1]
		recursersList = recursersList[:len(recursersList)-1]
		log.Printf("%s was the odd-one-out today", recurser.Name)

		err := pl.zulip.SendUserMessage(ctx, []int64{recurser.ID}, oddOneOutMessage)
		if err != nil {
			log.Printf("Error when trying to send oddOneOut message to %s: %s\n", recurser.Name, err)
		}
	}

	for i := 0; i < len(recursersList); i += 2 {
		rc1 := recursersList[i]
		rc2 := recursersList[i+1]
		ids := []int64{rc1.ID, rc2.ID}

		err := pl.zulip.SendUserMessage(ctx, ids, matchedMessage)
		if err != nil {
			log.Printf("Error when trying to send matchedMessage to %s and %s: %s\n", rc1.Name, rc2.Name, err)
		}
		log.Println(rc1.Name, "was", "matched", "with", rc2.Name)
	}

	numRecursersPairedUp := len(recursersList)

	log.Printf("Pairing Bot paired up %d recursers today", numRecursersPairedUp)

	numPairings := numRecursersPairedUp / 2

	timestamp := time.Now().Unix()
	if err := pl.pdb.SetNumPairings(ctx, int(timestamp), numPairings); err != nil {
		log.Printf("Failed to record today's pairings: %s", err)
	}
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

	profiles, err := pl.recurse.ActiveRecursers(ctx)
	if err != nil {
		log.Println("Encountered error while getting currently-active Recursers: ", err)
		// TODO: https://github.com/recursecenter/pairing-bot/issues/61: Alert here!
		// Using a FATAL here so it gets called out in the logs.
		log.Fatal("Aborting end-of-batch processing!")
		return
	}

	var idsOfPeopleAtRc []int64
	for _, p := range profiles {
		idsOfPeopleAtRc = append(idsOfPeopleAtRc, p.ZulipID)
	}

	for i := 0; i < len(recursersList); i++ {

		recurser := &recursersList[i]

		isAtRCThisWeek := contains(idsOfPeopleAtRc, recurser.ID)
		wasAtRCLastWeek := recursersList[i].CurrentlyAtRC

		log.Printf("User: %s was at RC last week: %t and is at RC this week: %t", recurser.Name, wasAtRCLastWeek, isAtRCThisWeek)

		recurser.CurrentlyAtRC = isAtRCThisWeek

		if err = pl.rdb.Set(ctx, recurser.ID, recurser); err != nil {
			log.Printf("Error encountered while update currentlyAtRC status for user: %s (ID %d)", recurser.Name, recurser.ID)
		}

		// If they were at RC last week but not this week then we assume they have graduated or otherwise left RC
		// In that case we remove them from pairing bot so that inactive people do not get matched
		// If people who have left RC still want to use pairing bot, we give them the option to resubscribe
		if wasAtRCLastWeek && !isAtRCThisWeek {
			var message string

			err = pl.rdb.Delete(ctx, recurser.ID)
			if err != nil {
				log.Println(err)
				message = fmt.Sprintf("Uh oh, I was trying to offboard you since it's the end of batch, but something went wrong. Consider messaging the maintainers to let them know this happened: %s", maintainersMention())
			} else {
				log.Printf("This user has been unsubscribed from pairing bot: %s (ID: %d)", recurser.Name, recurser.ID)

				message = offboardedMessage
			}

			err := pl.zulip.SendUserMessage(ctx, []int64{recurser.ID}, message)
			if err != nil {
				log.Printf("Error when trying to send offboarding message to %s (ID %d): %s", recurser.Name, recurser.ID, err)
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

	checkinMessage, err := renderCheckin(time.Now(), numPairings, len(recursersList), review.content)
	if err != nil {
		log.Printf("Error when trying to render Pairing Bot checkin: %s", err)
		return
	}

	if err := pl.zulip.PostToTopic(ctx, "checkins", "Pairing Bot", checkinMessage); err != nil {
		log.Printf("Error when trying to submit Pairing Bot checkins stream message: %s\n", err)
	}
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

	batches, err := pl.recurse.AllBatches(ctx)
	if err != nil {
		log.Printf("Error when fetching batches: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Loop through the batches until we find the first non-mini batch. Mini
	// batches are only 1 week long, so it doesn't make sense to send a message
	// 1 week after a mini batch has started :joy:
	var currentBatch recurse.Batch
	for _, batch := range batches {
		if batch.IsMini() {
			continue
		}

		currentBatch = batch
		break
	}

	now := time.Now()
	if currentBatch.IsSecondWeek(now) {
		msg, err := renderWelcome(now)
		if err != nil {
			log.Printf("Error when trying to send welcome message about Pairing Bot %s\n", err)
			return
		}

		if err := pl.zulip.PostToTopic(ctx, "397 Bridge", "🍐🤖", msg); err != nil {
			log.Printf("Error when trying to send welcome message about Pairing Bot %s\n", err)
		}
	}
}
