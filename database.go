package main

import (
	"context"
	"log"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// this is what we send to / receive from Firestore
// var recurser = map[string]interface{}{
// 	"id":                 "string",
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
	id                 string
	name               string
	email              string
	isSkippingTomorrow bool
	schedule           map[string]interface{}
	isSubscribed       bool
	currentlyAtRC      bool
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

func MapToStruct(m map[string]interface{}) Recurser {
	// isSubscribed is missing here because it's not in the map
	return Recurser{id: m["id"].(string),
		name:               m["name"].(string),
		email:              m["email"].(string),
		isSkippingTomorrow: m["isSkippingTomorrow"].(bool),
		schedule:           m["schedule"].(map[string]interface{}),
		currentlyAtRC:      m["currentlyAtRC"].(bool),
	}
}

// DB Lookups of Pairing Bot subscribers (= "Recursers")

type RecurserDB interface {
	GetByUserID(ctx context.Context, userID, userEmail, userName string) (Recurser, error)
	GetAllUsers(ctx context.Context) ([]Recurser, error)
	Set(ctx context.Context, userID string, recurser Recurser) error
	Delete(ctx context.Context, userID string) error
	ListPairingTomorrow(ctx context.Context) ([]Recurser, error)
	ListSkippingTomorrow(ctx context.Context) ([]Recurser, error)
	UnsetSkippingTomorrow(ctx context.Context, recurser Recurser) error
}

// implements RecurserDB
type FirestoreRecurserDB struct {
	client *firestore.Client
}

func (f *FirestoreRecurserDB) GetByUserID(ctx context.Context, userID, userEmail, userName string) (Recurser, error) {
	// get the users "document" (database entry) out of firestore
	// we temporarily keep it in 'doc'
	doc, err := f.client.Collection("recursers").Doc(userID).Get(ctx)
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
		recurser := doc.Data()
		recurser["name"] = userName
		recurser["email"] = userEmail
		r = MapToStruct(recurser)
	} else {
		// User is not subscribed, so provide a default recurser struct instead.
		r = Recurser{
			id:                 userID,
			name:               userName,
			email:              userEmail,
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
	// now put the data from the recurser map into a Recurser struct
	r.isSubscribed = isSubscribed
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
		r = MapToStruct(doc.Data())

		recursersList = append(recursersList, r)
	}
	return recursersList, nil
}

func (f *FirestoreRecurserDB) Set(ctx context.Context, userID string, recurser Recurser) error {

	r := recurser.ConvertToMap()
	_, err := f.client.Collection("recursers").Doc(userID).Set(ctx, r, firestore.MergeAll)
	return err

}

func (f *FirestoreRecurserDB) Delete(ctx context.Context, userID string) error {
	_, err := f.client.Collection("recursers").Doc(userID).Delete(ctx)
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

		r = MapToStruct(doc.Data())

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

		r = MapToStruct(doc.Data())

		skippersList = append(skippersList, r)
	}
	return skippersList, nil
}

func (f *FirestoreRecurserDB) UnsetSkippingTomorrow(ctx context.Context, recurser Recurser) error {

	r := recurser.ConvertToMap()
	r["isSkippingTomorrow"] = false

	_, err := f.client.Collection("recursers").Doc(r["id"].(string)).Set(ctx, r, firestore.MergeAll)
	return err
}

// implements RecurserDB
type MockRecurserDB struct{}

func (m *MockRecurserDB) GetByUserID(ctx context.Context, userID, userEmail, userName string) (Recurser, error) {
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
	SetKey(ctx context.Context, col string, doc string, value int) error
	GetKey(ctx context.Context, col string, doc string) (int, error)
	SetNumPairings(ctx context.Context, day string, numPairings int) error
	GetTotalPairingsDuringLastWeek(ctx context.Context) (int, error)
}

// implements RecurserDB
type FirestorePairingsDB struct {
	client *firestore.Client
}

func (f *FirestorePairingsDB) SetKey(ctx context.Context, col string, doc string, value int) error {
	_, err := f.client.Collection(col).Doc(doc).Set(ctx, map[string]int{
		"value": value,
	})
	return err
}

func (f *FirestorePairingsDB) GetKey(ctx context.Context, col string, doc string) (int, error) {
	res, err := f.client.Collection(col).Doc(doc).Get(ctx)
	if err != nil {
		return 0, err
	}

	token := res.Data()
	return token["value"].(int), nil
}

func (f *FirestorePairingsDB) SetNumPairings(ctx context.Context, day string, numPairings int) error {
	return f.SetKey(ctx, "pairings", day, numPairings)
}

func (f *FirestorePairingsDB) GetTotalPairingsDuringLastWeek(ctx context.Context) (int, error) {

	totalPairings := 0

	iter := f.client.Collection("pairings").Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return 0, err
		}

		log.Println("This is the value of the current document", doc.Data()["value"])

		dailyPairings := doc.Data()["value"].(int)

		totalPairings += dailyPairings
	}

	log.Println(totalPairings)

	return totalPairings, nil
}
