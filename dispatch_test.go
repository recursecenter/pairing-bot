package main

import (
	"context"
	"testing"

	"github.com/recursecenter/pairing-bot/internal/pbtest"
)

func Test_dispatch(t *testing.T) {
	ctx := context.Background()
	client := pbtest.FirestoreClient(t, ctx)

	pl := &PairingLogic{
		rdb:     &RecursersClient{client},
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
