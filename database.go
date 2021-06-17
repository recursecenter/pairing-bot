package main

import (
	"context"

	"cloud.google.com/go/firestore"
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
// }

type Recurser struct {
	id                 string
	name               string
	email              string
	isSkippingTomorrow bool
	schedule           map[string]interface{}
	isSubscribed       bool
}

func (r *Recurser) ConvertToMap() map[string]interface{} {
	return map[string]interface{}{
		"id":                 r.id,
		"name":               r.name,
		"email":              r.email,
		"isSkippingTomorrow": r.isSkippingTomorrow,
		"schedule":           r.schedule,
	}
}

func MapToStruct(m map[string]interface{}) Recurser {
	// isSubscribed is missing here because it's not in the map
	return Recurser{id: m["id"].(string),
		name:               m["name"].(string),
		email:              m["email"].(string),
		isSkippingTomorrow: m["isSkippingTomorrow"].(bool),
		schedule:           m["schedule"].(map[string]interface{}),
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
	return Recurser{}, nil
}

func (f *FirestoreRecurserDB) GetAllUsers(ctx context.Context) ([]Recurser, error) {
	return nil, nil
}

func (f *FirestoreRecurserDB) Set(ctx context.Context, userID string, recurser Recurser) error {
	return nil
}

func (f *FirestoreRecurserDB) Delete(ctx context.Context, userID string) error {
	return nil
}

func (f *FirestoreRecurserDB) ListPairingTomorrow(ctx context.Context) ([]Recurser, error) {
	return nil, nil
}

func (f *FirestoreRecurserDB) ListSkippingTomorrow(ctx context.Context) ([]Recurser, error) {
	return nil, nil
}

func (f *FirestoreRecurserDB) UnsetSkippingTomorrow(ctx context.Context, recurser Recurser) error {
	return nil
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
	return "", nil
}

// implements APIAuthDB
type MockAPIAuthDB struct{}

func (f *MockAPIAuthDB) GetKey(ctx context.Context, col, doc string) (string, error) {
	return "", nil
}
