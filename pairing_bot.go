package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"slices"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/recursecenter/pairing-bot/recurse"
	"github.com/recursecenter/pairing-bot/store"
	"github.com/recursecenter/pairing-bot/zulip"
)

var maintenanceMode = false

// maintainers contains the Zulip IDs of the current maintainers.
//
// This is a map instead of a slice to allow for easy membership checks.
var maintainers = map[int64]struct{}{
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
	db      *firestore.Client
	zulip   *zulip.Client
	recurse *recurse.Client

	version       string
	welcomeStream string
}

func (pl *PairingLogic) handle(w http.ResponseWriter, r *http.Request) {
	var err error

	responder := json.NewEncoder(w)

	// check and authorize the incoming request
	// observation: we only validate requests for /webhooks, i.e. user input through zulip

	ctx := r.Context()

	log.Println("Handling a new Zulip request")

	botAuth, err := store.Secrets(pl.db).Get(ctx, "zulip_webhook_token")
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

	user, err := store.Recursers(pl.db).GetByUserID(ctx, hook.Message.SenderID, hook.Message.SenderEmail, hook.Message.SenderFullName)
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

// Match generates new pairs for today and sends notifications for them.
func (pl *PairingLogic) Match(ctx context.Context) error {
	recursersList, err := store.Recursers(pl.db).ListPairingTomorrow(ctx)
	log.Println(recursersList)
	if err != nil {
		return fmt.Errorf("get today's recursers from DB: %w", err)
	}

	skippersList, err := store.Recursers(pl.db).ListSkippingTomorrow(ctx)
	if err != nil {
		return fmt.Errorf("get today's skippers from DB: %w", err)
	}

	// get everyone who was set to skip today and set them back to isSkippingTomorrow = false
	for _, skipper := range skippersList {
		err := store.Recursers(pl.db).UnsetSkippingTomorrow(ctx, &skipper)
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
		return nil
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

	pairing := store.Pairing{
		Value:     numRecursersPairedUp / 2,
		Timestamp: time.Now().Unix(),
	}

	if err := store.Pairings(pl.db).SetNumPairings(ctx, pairing); err != nil {
		log.Printf("Failed to record today's pairings: %s", err)
	}

	return nil
}

// EndOfBatch unsubscribes everyone who just never-graduated with this batch.
func (pl *PairingLogic) EndOfBatch(ctx context.Context) error {
	// getting all the recursers
	recursersList, err := store.Recursers(pl.db).GetAllUsers(ctx)
	if err != nil {
		log.Println("Could not get list of recursers from DB: ", err)
	}

	profiles, err := pl.recurse.ActiveRecursers(ctx)
	if err != nil {
		return fmt.Errorf("get active Recursers: %w", err)
	}

	var idsOfPeopleAtRc []int64
	for _, p := range profiles {
		idsOfPeopleAtRc = append(idsOfPeopleAtRc, p.ZulipID)
	}

	for i := 0; i < len(recursersList); i++ {

		recurser := &recursersList[i]

		isAtRCThisWeek := slices.Contains(idsOfPeopleAtRc, recurser.ID)
		wasAtRCLastWeek := recursersList[i].CurrentlyAtRC

		log.Printf("User: %s was at RC last week: %t and is at RC this week: %t", recurser.Name, wasAtRCLastWeek, isAtRCThisWeek)

		recurser.CurrentlyAtRC = isAtRCThisWeek

		if err = store.Recursers(pl.db).Set(ctx, recurser.ID, recurser); err != nil {
			log.Printf("Error encountered while update currentlyAtRC status for user: %s (ID %d)", recurser.Name, recurser.ID)
		}

		// If they were at RC last week but not this week then we assume they have graduated or otherwise left RC
		// In that case we remove them from pairing bot so that inactive people do not get matched
		// If people who have left RC still want to use pairing bot, we give them the option to resubscribe
		if wasAtRCLastWeek && !isAtRCThisWeek {
			var message string

			err = store.Recursers(pl.db).Delete(ctx, recurser.ID)
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

	return nil
}

// Checkin posts a message to Pairing Bot's checkin topic.
func (pl *PairingLogic) Checkin(ctx context.Context) error {
	numPairings, err := store.Pairings(pl.db).GetTotalPairingsDuringLastWeek(ctx)
	if err != nil {
		log.Println("Unable to get the total number of pairings during the last week: : ", err)
	}

	recursersList, err := store.Recursers(pl.db).GetAllUsers(ctx)
	if err != nil {
		log.Printf("Could not get list of recursers from DB: %s\n", err)
	}

	review, err := store.Reviews(pl.db).GetRandom(ctx)
	if err != nil {
		log.Println("Could not get a random review from DB: ", err)
	}

	checkinMessage, err := renderCheckin(time.Now(), numPairings, len(recursersList), review.Content)
	if err != nil {
		return fmt.Errorf("render checkin: %w", err)
	}

	if err := pl.zulip.PostToTopic(ctx, "checkins", "Pairing Bot", checkinMessage); err != nil {
		return fmt.Errorf("send checkin: %w", err)
	}

	return nil
}

// Welcome sends a "Welcome to Pairing Bot" message to introduce the new batch
// to Pairing Bot.
//
// We send this message during the second week of batch. The first week is a
// bit overwhelming with all of the orientation meetings and messages, and
// people haven't had time to think too much about their projects.
func (pl *PairingLogic) Welcome(ctx context.Context) error {
	batches, err := pl.recurse.AllBatches(ctx)
	if err != nil {
		return fmt.Errorf("get list of batches: %w", err)
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
			return fmt.Errorf("render welcome message: %w", err)
		}

		if err := pl.zulip.PostToTopic(ctx, pl.welcomeStream, "🍐🤖", msg); err != nil {
			return fmt.Errorf("send welcome message: %w", err)
		}
	}

	return nil
}
