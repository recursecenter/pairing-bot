package main

import (
	"context"
	"testing"
)

func Test_dispatch(t *testing.T) {
	ctx := context.Background()
	projectID := fakeProjectID(t)

	client := testFirestoreClient(t, ctx, projectID)

	pl := &PairingLogic{
		rdb:     &FirestoreRecurserDB{client},
		version: "test string",
	}

	rec := &Recurser{
		ID:                 0,
		Name:               "Your Name",
		Email:              "fake@recurse.example.net",
		IsSkippingTomorrow: false,
		Schedule:           map[string]bool{},
		CurrentlyAtRC:      false,
		IsSubscribed:       false,
	}

	t.Run("version", func(t *testing.T) {
		resp, err := pl.dispatch(ctx, "version", nil, rec)
		if err != nil {
			t.Fatal(err)
		}

		expected := "test string"
		if resp != expected {
			t.Errorf("expected %q, got %q", expected, resp)
		}
	})
}
