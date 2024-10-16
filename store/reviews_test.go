package store_test

import (
	"context"
	"testing"

	"github.com/recursecenter/pairing-bot/internal/assert"
	"github.com/recursecenter/pairing-bot/internal/pbtest"
	"github.com/recursecenter/pairing-bot/store"
)

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
