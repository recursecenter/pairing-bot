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

// this is what we send to / receive from Firestore
// var recurser = map[string]interface{}{
// 	"id":                 1234,
// 	"name":               "string",
// 	"email":              "string",
// 	"isSkippingTomorrow": false,
// 	"schedule": map[string]interface{}{
// 		"monday":    false,
// 		"tuesday":   false,
// 		"wednesday": false,
// 		"thursday":  false,
// 		"friday":    false,
// 		"saturday":  false,
// 		"sunday":    false,
// 	},
//  "currentlyAtRC":      false,
// }

type Recurser struct {
	id                 int64
	name               string
	email              string
	isSkippingTomorrow bool
	schedule           map[string]interface{}

	// isSubscribed really means "did they already have an entry in the database"
	isSubscribed  bool
	currentlyAtRC bool
}

func (r *Recurser) ConvertToMap() map[string]interface{} {
	return map[string]interface{}{
		"id":                 r.id,
		"name":               r.name,
		"email":              r.email,
		"isSkippingTomorrow": r.isSkippingTomorrow,
		"schedule":           r.schedule,
		"currentlyAtRC":      r.currentlyAtRC,
	}
}

func parseDoc(doc *firestore.DocumentSnapshot) (Recurser, error) {
	id, err := strconv.ParseInt(doc.Ref.ID, 10, 64)
	if err != nil {
		return Recurser{}, fmt.Errorf("invalid ID value: %w", err)
	}
	m := doc.Data()

	// isSubscribed is missing here because it's not in the map
	return Recurser{
		id: id,
		// isSubscribed is implicit by the existence of an entry in the database.
		isSubscribed:       true,
		name:               m["name"].(string),
		email:              m["email"].(string),
		isSkippingTomorrow: m["isSkippingTomorrow"].(bool),
		schedule:           m["schedule"].(map[string]interface{}),
		currentlyAtRC:      m["currentlyAtRC"].(bool),
	}, nil
}

// DB Lookups of Pairing Bot subscribers (= "Recursers")

type RecurserDB interface {
	GetByUserID(ctx context.Context, userID int64, userEmail, userName string) (Recurser, error)
	GetAllUsers(ctx context.Context) ([]Recurser, error)
	Set(ctx context.Context, userID int64, recurser Recurser) error
	Delete(ctx context.Context, userID int64) error
	ListPairingTomorrow(ctx context.Context) ([]Recurser, error)
	ListSkippingTomorrow(ctx context.Context) ([]Recurser, error)
	UnsetSkippingTomorrow(ctx context.Context, recurser Recurser) error
}

// implements RecurserDB
type FirestoreRecurserDB struct {
	client *firestore.Client
}

func (f *FirestoreRecurserDB) GetByUserID(ctx context.Context, userID int64, userEmail, userName string) (Recurser, error) {
	stringID := fmt.Sprintf("%d", userID)
	// get the users "document" (database entry) out of firestore
	// we temporarily keep it in 'doc'
	doc, err := f.client.Collection("recursers").Doc(stringID).Get(ctx)
	// this says "if there's an error, and if that error was not document-not-found"
	if err != nil && status.Code(err) != codes.NotFound {
		return Recurser{}, err
	}

	// if there's a db entry, that means they were already subscribed to pairing bot
	// if there's not, they were not subscribed
	isSubscribed := doc.Exists()

	var r Recurser
	// if the user is in the database, get their current state into this map
	// also assign their zulip name to the name field, just in case it changed
	// also assign their email, for the same reason
	if isSubscribed {
		r, err = parseDoc(doc)
		if err != nil {
			return Recurser{}, fmt.Errorf("parsing database entry for recurser %s (ID: %d): %w", userName, userID, err)
		}
		// Update with the latest from Zulip:
		r.name = userName
		r.email = userEmail
	} else {
		// User is not subscribed, so provide a default recurser struct instead.
		r = Recurser{
			id:    userID,
			name:  userName,
			email: userEmail,

			// They didn't already have an entry, so they are not considered subscribed.
			isSubscribed:       false,
			isSkippingTomorrow: false,
			schedule: map[string]interface{}{
				"monday":    true,
				"tuesday":   true,
				"wednesday": true,
				"thursday":  true,
				"friday":    true,
				"saturday":  false,
				"sunday":    false,
			},
			currentlyAtRC: true,
		}
	}
	return r, nil
}

func (f *FirestoreRecurserDB) GetAllUsers(ctx context.Context) ([]Recurser, error) {

	var recursersList []Recurser
	var r Recurser

	iter := f.client.Collection("recursers").Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		r, err = parseDoc(doc)
		if err != nil {
			log.Printf("error parsing database entry with ID: %s: %s", doc.Ref.ID, err)
			log.Printf("skipping database entry (ID: %s)", doc.Ref.ID)
			continue
		}

		recursersList = append(recursersList, r)
	}
	return recursersList, nil
}

func (f *FirestoreRecurserDB) Set(ctx context.Context, userID int64, recurser Recurser) error {

	r := recurser.ConvertToMap()
	_, err := f.client.Collection("recursers").Doc(fmt.Sprintf("%d", userID)).Set(ctx, r, firestore.MergeAll)
	return err

}

func (f *FirestoreRecurserDB) Delete(ctx context.Context, userID int64) error {
	_, err := f.client.Collection("recursers").Doc(fmt.Sprintf("%d", userID)).Delete(ctx)
	return err
}

func (f *FirestoreRecurserDB) ListPairingTomorrow(ctx context.Context) ([]Recurser, error) {
	// this gets the time from system time, which is UTC
	// on app engine (and most other places). This works
	// fine for us in NYC, but might not if pairing bot
	// were ever running in another time zone
	today := strings.ToLower(time.Now().Weekday().String())

	var recursersList []Recurser
	var r Recurser

	// ok this is how we have to get all the recursers. it's weird.
	// this query returns an iterator, and then we have to use firestore
	// magic to iterate across the results of the query and store them
	// into our 'recursersList' variable which is a slice of map[string]interface{}
	iter := f.client.Collection("recursers").Where("isSkippingTomorrow", "==", false).Where("schedule."+today, "==", true).Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		r, err = parseDoc(doc)
		if err != nil {
			log.Printf("error parsing database entry with ID: %s: %s", doc.Ref.ID, err)
			log.Printf("skipping database entry (ID: %s)", doc.Ref.ID)
			continue
		}

		recursersList = append(recursersList, r)
	}

	return recursersList, nil
}

func (f *FirestoreRecurserDB) ListSkippingTomorrow(ctx context.Context) ([]Recurser, error) {

	var skippersList []Recurser
	var r Recurser

	iter := f.client.Collection("recursers").Where("isSkippingTomorrow", "==", true).Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		r, err = parseDoc(doc)
		if err != nil {
			log.Printf("error parsing database entry with ID: %s: %s", doc.Ref.ID, err)
			log.Printf("skipping database entry (ID: %s)", doc.Ref.ID)
			continue
		}

		skippersList = append(skippersList, r)
	}
	return skippersList, nil
}

func (f *FirestoreRecurserDB) UnsetSkippingTomorrow(ctx context.Context, recurser Recurser) error {

	r := recurser.ConvertToMap()
	r["isSkippingTomorrow"] = false

	// There are two IDs that have to match:
	//  1. The recurser ID *inside* the document, which is an int64.
	//  2. The document ID *itself*, which is a string.
	rID, ok := r["id"].(int64)
	if !ok {
		return fmt.Errorf(`recurser's "id" field was not an int64: %+v`, r)
	}

	docID := strconv.FormatInt(rID, 10)
	_, err := f.client.Collection("recursers").Doc(docID).Set(ctx, r, firestore.MergeAll)
	return err
}

// implements RecurserDB
type MockRecurserDB struct{}

func (m *MockRecurserDB) GetByUserID(ctx context.Context, userID int64, userEmail, userName string) (Recurser, error) {
	return Recurser{}, nil
}

func (m *MockRecurserDB) GetAllUsers(ctx context.Context) ([]Recurser, error) {
	return nil, nil
}

func (m *MockRecurserDB) Set(ctx context.Context, userID string, recurser Recurser) error {
	return nil
}

func (m *MockRecurserDB) Delete(ctx context.Context, userID string) error {
	return nil
}

func (m *MockRecurserDB) ListPairingTomorrow(ctx context.Context) ([]Recurser, error) {
	return nil, nil
}

func (m *MockRecurserDB) ListSkippingTomorrow(ctx context.Context) ([]Recurser, error) {
	return nil, nil
}

func (m *MockRecurserDB) UnsetSkippingTomorrow(ctx context.Context, userID string) error {
	return nil
}

// DB Lookups of tokens

type APIAuthDB interface {
	GetKey(ctx context.Context, col, doc string) (string, error)
}

// implements APIAuthDB
type FirestoreAPIAuthDB struct {
	client *firestore.Client
}

func (f *FirestoreAPIAuthDB) GetKey(ctx context.Context, col, doc string) (string, error) {
	res, err := f.client.Collection(col).Doc(doc).Get(ctx)
	if err != nil {
		return "", err
	}

	token := res.Data()
	return token["value"].(string), nil
}

// implements APIAuthDB
type MockAPIAuthDB struct{}

func (f *MockAPIAuthDB) GetKey(ctx context.Context, col, doc string) (string, error) {
	return "", nil
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
	content   string
	email     string
	timestamp int
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

		currentReview := Review{
			content:   doc.Data()["content"].(string),
			email:     doc.Data()["email"].(string),
			timestamp: int(doc.Data()["timestamp"].(int64)),
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

		currentReview := Review{
			content:   doc.Data()["content"].(string),
			email:     doc.Data()["email"].(string),
			timestamp: int(doc.Data()["timestamp"].(int64)),
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
	_, _, err := f.client.Collection("reviews").Add(ctx, map[string]interface{}{
		"content":   review.content,
		"email":     review.email,
		"timestamp": review.timestamp,
	})
	return err
}
