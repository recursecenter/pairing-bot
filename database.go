package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// A Schedule determines whether to pair a Recurser on each day.
// The only valid keys are all-lowercase day names (e.g., "monday").
type Schedule map[string]bool

func defaultSchedule() map[string]bool {
	return map[string]bool{
		"monday":    true,
		"tuesday":   true,
		"wednesday": true,
		"thursday":  true,
		"friday":    true,

		"saturday": false,
		"sunday":   false,
	}
}

func newSchedule(days []string) map[string]bool {
	schedule := emptySchedule()
	for _, day := range days {
		schedule[day] = true
	}
	return schedule
}

func emptySchedule() map[string]bool {
	return map[string]bool{
		"monday":    false,
		"tuesday":   false,
		"wednesday": false,
		"thursday":  false,
		"friday":    false,
		"saturday":  false,
		"sunday":    false,
	}
}

type Recurser struct {
	ID                 int64           `firestore:"id"`
	Name               string          `firestore:"name"`
	Email              string          `firestore:"email"`
	IsSkippingTomorrow bool            `firestore:"isSkippingTomorrow"`
	Schedule           map[string]bool `firestore:"schedule"`
	CurrentlyAtRC      bool            `firestore:"currentlyAtRC"`

	// IsSubscribed really means "already had an entry in the database".
	// It is not written to or read from the Firestore document.
	IsSubscribed bool `firestore:"-"`
}

// FirestoreRecurserDB manages Pairing Bot subscribers ("Recursers").
type FirestoreRecurserDB struct {
	client *firestore.Client
}

func (f *FirestoreRecurserDB) GetByUserID(ctx context.Context, userID int64, userEmail, userName string) (*Recurser, error) {
	docID := strconv.FormatInt(userID, 10)
	doc, err := f.client.Collection("recursers").Doc(docID).Get(ctx)

	// A missing document still returns a non-nil doc with its NotFound error.
	// Any other error is a real error.
	if err != nil && status.Code(err) != codes.NotFound {
		return nil, err
	}

	if !doc.Exists() {
		// If we don't have a record, that just means they're not subscribed.
		return &Recurser{
			ID:       userID,
			Name:     userName,
			Email:    userEmail,
			Schedule: defaultSchedule(),
		}, nil
	}

	var recurser Recurser
	if err := doc.DataTo(&recurser); err != nil {
		return nil, fmt.Errorf("parse document %q: %w", doc.Ref.Path, err)
	}

	// This field isn't stored in the DB, so populate it now.
	recurser.IsSubscribed = true

	// Prefer the Zulip values for these fields over our cached ones.
	recurser.Name = userName
	recurser.Email = userEmail

	return &recurser, nil
}

func (f *FirestoreRecurserDB) GetAllUsers(ctx context.Context) ([]Recurser, error) {
	iter := f.client.Collection("recursers").Documents(ctx)
	return fetchAll[Recurser](iter)
}

func (f *FirestoreRecurserDB) Set(ctx context.Context, _ int64, recurser *Recurser) error {
	docID := strconv.FormatInt(recurser.ID, 10)

	// Merging isn't supported when using struct data, but we never do partial
	// writes in the first place. So this will completely overwrite an existing
	// document.
	_, err := f.client.Collection("recursers").Doc(docID).Set(ctx, recurser)
	return err

}

func (f *FirestoreRecurserDB) Delete(ctx context.Context, userID int64) error {
	docID := strconv.FormatInt(userID, 10)
	_, err := f.client.Collection("recursers").Doc(docID).Delete(ctx)
	return err
}

func (f *FirestoreRecurserDB) ListPairingTomorrow(ctx context.Context) ([]Recurser, error) {
	// this gets the time from system time, which is UTC
	// on app engine (and most other places). This works
	// fine for us in NYC, but might not if pairing bot
	// were ever running in another time zone
	today := strings.ToLower(time.Now().Weekday().String())

	iter := f.client.
		Collection("recursers").
		Where("isSkippingTomorrow", "==", false).
		Where("schedule."+today, "==", true).
		Documents(ctx)
	return fetchAll[Recurser](iter)
}

func (f *FirestoreRecurserDB) ListSkippingTomorrow(ctx context.Context) ([]Recurser, error) {
	iter := f.client.
		Collection("recursers").
		Where("isSkippingTomorrow", "==", true).
		Documents(ctx)
	return fetchAll[Recurser](iter)
}

func (f *FirestoreRecurserDB) UnsetSkippingTomorrow(ctx context.Context, recurser *Recurser) error {
	recurser.IsSkippingTomorrow = false
	return f.Set(ctx, recurser.ID, recurser)
}

// FirestoreAPIAuthDB manages auth tokens stored in Firestore.
type FirestoreAPIAuthDB struct {
	client *firestore.Client
}

func (f *FirestoreAPIAuthDB) GetToken(ctx context.Context, path string) (string, error) {
	doc, err := f.client.Doc(path).Get(ctx)
	if err != nil {
		return "", err
	}

	var token struct {
		Value string `firestore:"value"`
	}

	if err := doc.DataTo(&token); err != nil {
		return "", err
	}

	return token.Value, nil
}

type PairingsDB interface {
	SetNumPairings(ctx context.Context, timestamp int, numPairings int) error
	GetTotalPairingsDuringLastWeek(ctx context.Context) (int, error)
}

// implements RecurserDB
type FirestorePairingsDB struct {
	client *firestore.Client
}

func (f *FirestorePairingsDB) SetNumPairings(ctx context.Context, timestamp int, numPairings int) error {
	timestampAsString := strconv.Itoa(timestamp)

	_, err := f.client.Collection("pairings").Doc(timestampAsString).Set(ctx, map[string]interface{}{
		"value":     numPairings,
		"timestamp": timestamp,
	})
	return err
}

func (f *FirestorePairingsDB) GetTotalPairingsDuringLastWeek(ctx context.Context) (int, error) {

	totalPairings := 0

	timestampSevenDaysAgo := time.Now().Add(-7 * 24 * time.Hour).Unix()

	iter := f.client.Collection("pairings").Where("timestamp", ">", timestampSevenDaysAgo).Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return 0, err
		}

		dailyPairings := int(doc.Data()["value"].(int64))

		log.Println("The timestamp is: ", doc.Data()["timestamp"])

		totalPairings += dailyPairings
	}

	return totalPairings, nil
}

type Review struct {
	Content   string `firestore:"content"`
	Email     string `firestore:"email"`
	Timestamp int64  `firestore:"timestamp"`
}

type ReviewDB interface {
	GetAll(ctx context.Context) ([]Review, error)
	GetLastN(ctx context.Context, n int) ([]Review, error)
	GetRandom(ctx context.Context) (Review, error)
	Insert(ctx context.Context, review Review) error
}

// implements ReviewDB
type FirestoreReviewDB struct {
	client *firestore.Client
}

func (f *FirestoreReviewDB) GetAll(ctx context.Context) ([]Review, error) {
	var allReviews []Review

	iter := f.client.Collection("reviews").Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		var currentReview Review
		if err := doc.DataTo(&currentReview); err != nil {
			// TODO: log skip
			continue
		}

		allReviews = append(allReviews, currentReview)
	}

	return allReviews, nil
}

func (f *FirestoreReviewDB) GetLastN(ctx context.Context, n int) ([]Review, error) {
	var lastFive []Review

	iter := f.client.Collection("reviews").OrderBy("timestamp", firestore.Desc).Limit(n).Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		var currentReview Review
		if err := doc.DataTo(&currentReview); err != nil {
			// TODO: log skip
			continue
		}

		lastFive = append(lastFive, currentReview)
	}

	return lastFive, nil
}

func (f *FirestoreReviewDB) GetRandom(ctx context.Context) (Review, error) {
	allReviews, err := f.GetAll(ctx)

	if err != nil {
		return Review{}, err
	}

	return allReviews[rand.Intn(len(allReviews))], nil
}

func (f *FirestoreReviewDB) Insert(ctx context.Context, review Review) error {
	_, _, err := f.client.Collection("reviews").Add(ctx, review)
	return err
}

// fetchAll converts all documents in iter to values of type T. Documents that
// cannot be converted will be skipped.
//
// If the iterator yields an error instead of a document, this returns the
// first such error and stops.
func fetchAll[T any](iter *firestore.DocumentIterator) ([]T, error) {
	var all []T
	defer iter.Stop()

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			return all, nil
		} else if err != nil {
			return nil, err
		}

		var item T
		if err := doc.DataTo(&item); err != nil {
			log.Printf("Skipping %q: %s", doc.Ref.Path, err)
			continue
		}

		all = append(all, item)
	}
}
