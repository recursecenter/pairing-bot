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

// RecursersClient manages Pairing Bot subscribers ("Recursers").
type RecursersClient struct {
	client *firestore.Client
}

func Recursers(client *firestore.Client) *RecursersClient {
	return &RecursersClient{client}
}

func (r *RecursersClient) GetByUserID(ctx context.Context, userID int64, userEmail, userName string) (*Recurser, error) {
	docID := strconv.FormatInt(userID, 10)
	doc, err := r.client.Collection("recursers").Doc(docID).Get(ctx)

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

func (r *RecursersClient) GetAllUsers(ctx context.Context) ([]Recurser, error) {
	iter := r.client.Collection("recursers").Documents(ctx)
	return fetchAll[Recurser](iter)
}

func (r *RecursersClient) Set(ctx context.Context, _ int64, recurser *Recurser) error {
	docID := strconv.FormatInt(recurser.ID, 10)

	// Merging isn't supported when using struct data, but we never do partial
	// writes in the first place. So this will completely overwrite an existing
	// document.
	_, err := r.client.Collection("recursers").Doc(docID).Set(ctx, recurser)
	return err

}

func (r *RecursersClient) Delete(ctx context.Context, userID int64) error {
	docID := strconv.FormatInt(userID, 10)
	_, err := r.client.Collection("recursers").Doc(docID).Delete(ctx)
	return err
}

func (r *RecursersClient) ListPairingTomorrow(ctx context.Context) ([]Recurser, error) {
	// this gets the time from system time, which is UTC
	// on app engine (and most other places). This works
	// fine for us in NYC, but might not if pairing bot
	// were ever running in another time zone
	today := strings.ToLower(time.Now().Weekday().String())

	iter := r.client.
		Collection("recursers").
		Where("isSkippingTomorrow", "==", false).
		Where("schedule."+today, "==", true).
		Documents(ctx)
	return fetchAll[Recurser](iter)
}

func (r *RecursersClient) ListSkippingTomorrow(ctx context.Context) ([]Recurser, error) {
	iter := r.client.
		Collection("recursers").
		Where("isSkippingTomorrow", "==", true).
		Documents(ctx)
	return fetchAll[Recurser](iter)
}

func (r *RecursersClient) UnsetSkippingTomorrow(ctx context.Context, recurser *Recurser) error {
	recurser.IsSkippingTomorrow = false
	return r.Set(ctx, recurser.ID, recurser)
}

// SecretsClient manages auth tokens stored in Firestore.
type SecretsClient struct {
	client *firestore.Client
}

func Secrets(client *firestore.Client) *SecretsClient {
	return &SecretsClient{client}
}

func (s *SecretsClient) Get(ctx context.Context, name string) (string, error) {
	doc, err := s.client.Collection("secrets").Doc(name).Get(ctx)
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

type Pairing struct {
	Value     int   `firestore:"value"`
	Timestamp int64 `firestore:"timestamp"`
}

// PairingsClient manages pairing (matching) result records.
type PairingsClient struct {
	client *firestore.Client
}

func Pairings(client *firestore.Client) *PairingsClient {
	return &PairingsClient{client}
}

func (p *PairingsClient) SetNumPairings(ctx context.Context, pairing Pairing) error {
	timestampAsString := strconv.FormatInt(pairing.Timestamp, 10)

	_, err := p.client.Collection("pairings").Doc(timestampAsString).Set(ctx, pairing)
	return err
}

func (p *PairingsClient) GetTotalPairingsDuringLastWeek(ctx context.Context) (int, error) {
	totalPairings := 0

	timestampSevenDaysAgo := time.Now().Add(-7 * 24 * time.Hour).Unix()

	iter := p.client.Collection("pairings").Where("timestamp", ">", timestampSevenDaysAgo).Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return 0, err
		}

		var pairing Pairing
		if err = doc.DataTo(&pairing); err != nil {
			log.Printf("Skipping %q: %s", doc.Ref.Path, err)
			continue
		}

		log.Println("The timestamp is: ", pairing.Timestamp)

		totalPairings += pairing.Value
	}

	return totalPairings, nil
}

type Review struct {
	Content   string `firestore:"content"`
	Email     string `firestore:"email"`
	Timestamp int64  `firestore:"timestamp"`
}

// ReviewsClient manages user-submitted Pairing Bot reviews.
type ReviewsClient struct {
	client *firestore.Client
}

func Reviews(client *firestore.Client) *ReviewsClient {
	return &ReviewsClient{client}
}

func (r *ReviewsClient) GetAll(ctx context.Context) ([]Review, error) {
	iter := r.client.Collection("reviews").Documents(ctx)
	return fetchAll[Review](iter)
}

func (r *ReviewsClient) GetLastN(ctx context.Context, n int) ([]Review, error) {
	iter := r.client.
		Collection("reviews").
		OrderBy("timestamp", firestore.Desc).
		Limit(n).
		Documents(ctx)
	return fetchAll[Review](iter)
}

func (r *ReviewsClient) GetRandom(ctx context.Context) (Review, error) {
	allReviews, err := r.GetAll(ctx)

	if err != nil {
		return Review{}, err
	}

	return allReviews[rand.Intn(len(allReviews))], nil
}

func (r *ReviewsClient) Insert(ctx context.Context, review Review) error {
	_, _, err := r.client.Collection("reviews").Add(ctx, review)
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
