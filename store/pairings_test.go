package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/recursecenter/pairing-bot/internal/assert"
	"github.com/recursecenter/pairing-bot/internal/pbtest"
	"github.com/recursecenter/pairing-bot/store"
)

func TestFirestorePairingsClient(t *testing.T) {
	t.Run("round trip weekly pairings", func(t *testing.T) {
		ctx := context.Background()

		client := pbtest.FirestoreClient(t, ctx)
		pairings := store.Pairings(client)

		// Entries representing pairings for each day of the week
		for i := 6; i >= 0; i-- {
			err := pairings.SetNumPairings(ctx, store.Pairing{
				Value:     5,
				Timestamp: time.Now().Add(-time.Duration(i) * 24 * time.Hour).Unix(),
			})
			if err != nil {
				t.Fatal(err)
			}
		}

		expected := 35

		actual, err := pairings.GetTotalPairingsDuringLastWeek(ctx)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, actual, expected)
	})
}
