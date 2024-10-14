package store_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/recursecenter/pairing-bot/internal/assert"
	"github.com/recursecenter/pairing-bot/internal/pbtest"
	"github.com/recursecenter/pairing-bot/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestFirestoreRecursersClient(t *testing.T) {
	t.Run("round-trip new recurser", func(t *testing.T) {
		ctx := context.Background()

		client := pbtest.FirestoreClient(t, ctx)
		recursers := store.Recursers(client)

		recurser := store.Recurser{
			ID:                 pbtest.RandInt64(t),
			Name:               "Your Name",
			Email:              "test@recurse.example.net",
			IsSkippingTomorrow: false,
			Schedule:           store.NewSchedule([]string{"monday", "friday"}),
			IsSubscribed:       false,
			CurrentlyAtRC:      false,
		}

		err := recursers.Set(ctx, recurser.ID, &recurser)
		if err != nil {
			t.Fatal(err)
		}

		// GetByUserID forces isSubscribed to be `true`, because that's implied by
		// the record's existence in the DB in the first place.
		expected := recurser
		expected.IsSubscribed = true

		// GetByUserID will prefer the argument values for email and name if
		// they differ from what's stored in the DB. These values are the same,
		// so we wouldn't be able to tell from this call.
		unchanged, err := recursers.GetByUserID(ctx, recurser.ID, recurser.Email, recurser.Name)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, unchanged, &expected)

		// These values are different, so this call *does* tell us whether we
		// used the arguments.
		changed, err := recursers.GetByUserID(ctx, recurser.ID, "changed@recurse.example.net", "My Name")
		if err != nil {
			t.Fatal(err)
		}

		expected.Email = "changed@recurse.example.net"
		expected.Name = "My Name"

		assert.Equal(t, changed, &expected)

		// But none of this is actually stored in the DB. If we fetch the
		// collection directly, we can see the original name and email. And we
		// can see that IsSubscribed is false because it's not stored!
		doc, err := client.Collection("recursers").Doc(strconv.FormatInt(recurser.ID, 10)).Get(ctx)
		if err != nil {
			t.Fatal(err)
		}

		var actual store.Recurser
		if err := doc.DataTo(&actual); err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, actual, recurser)
	})
}

func TestFirestoreReviewsClient(t *testing.T) {
	t.Run("round-trip content", func(t *testing.T) {
		ctx := context.Background()

		client := pbtest.FirestoreClient(t, ctx)
		reviews := store.Reviews(client)

		review := store.Review{
			Content:   "test review",
			Email:     "test@recurse.example.net",
			Timestamp: pbtest.RandInt64(t),
		}

		err := reviews.Insert(ctx, review)
		if err != nil {
			t.Fatal(err)
		}

		// Reviews are returned as a slice, even for just one review
		expected := []store.Review{review}

		actual, err := reviews.GetLastN(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, actual, expected)
	})
}

func TestFirestoreSecretsClient(t *testing.T) {
	ctx := context.Background()

	client := pbtest.FirestoreClient(t, ctx)
	secrets := store.Secrets(client)

	// Try to keep tests from conflicting with each other by adding a token
	// that only this test knows about.
	key := fmt.Sprintf("token-%d", pbtest.RandInt64(t))
	val := fmt.Sprintf("secret-%d", pbtest.RandInt64(t))
	doc := map[string]any{
		"value": val,
	}
	_, err := client.Collection("secrets").Doc(key).Set(ctx, doc)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("missing", func(t *testing.T) {
		_, err := secrets.Get(ctx, "does-not-exist")
		if status.Code(err) != codes.NotFound {
			t.Fatalf("expected NotFound error, got %#+v", err)
		}
	})

	t.Run("present", func(t *testing.T) {
		actual, err := secrets.Get(ctx, key)
		if err != nil {
			t.Fatal(err)
		}

		if actual != val {
			t.Errorf("values not equal:\nactual:   %+v\nexpected: %+v", actual, val)
		}
	})
}

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
