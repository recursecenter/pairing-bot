package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"maps"
	"math/big"
	"strconv"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/recursecenter/pairing-bot/internal/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func fakeProjectID(t *testing.T) string {
	return fmt.Sprintf("fake-project-%d", randInt64(t))
}

func randInt64(t *testing.T) int64 {
	int64Max := int64(1<<63 - 1)

	n, err := rand.Int(rand.Reader, big.NewInt(int64Max))
	if err != nil {
		t.Fatal(err)
	}
	return n.Int64()
}

func testFirestoreClient(t *testing.T, ctx context.Context, projectID string) *firestore.Client {
	client, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Logf("Error closing Firestore client: %v", err)
		}
	})

	return client
}

func TestFirestoreRecurserDB(t *testing.T) {
	t.Run("round-trip new recurser", func(t *testing.T) {
		ctx := context.Background()
		projectID := fakeProjectID(t)

		client := testFirestoreClient(t, ctx, projectID)
		recursers := &FirestoreRecurserDB{client}

		recurser := Recurser{
			ID:                 randInt64(t),
			Name:               "Your Name",
			Email:              "test@recurse.example.net",
			IsSkippingTomorrow: false,
			Schedule:           newSchedule([]string{"monday", "friday"}),
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

		if !unchanged.Equal(expected) {
			t.Errorf("values not equal:\nactual:   %+v\nexpected: %+v", unchanged, expected)
		}

		// These values are different, so this call *does* tell us whether we
		// used the arguments.
		changed, err := recursers.GetByUserID(ctx, recurser.ID, "changed@recurse.example.net", "My Name")
		if err != nil {
			t.Fatal(err)
		}

		expected.Email = "changed@recurse.example.net"
		expected.Name = "My Name"

		if !changed.Equal(expected) {
			t.Errorf("values not equal:\nactual:   %+v\nexpected: %+v", changed, expected)
		}

		// But none of this is actually stored in the DB. If we fetch the
		// collection directly, we can see the original name and email. And we
		// can see that IsSubscribed is false because it's not stored!
		doc, err := client.Collection("recursers").Doc(strconv.FormatInt(recurser.ID, 10)).Get(ctx)
		if err != nil {
			t.Fatal(err)
		}

		var actual Recurser
		if err := doc.DataTo(&actual); err != nil {
			t.Fatal(err)
		}

		if !actual.Equal(recurser) {
			t.Errorf("values not equal:\nactual:   %+v\nexpected: %+v", actual, recurser)
		}
	})
}

func (r Recurser) Equal(s Recurser) bool {
	return r.ID == s.ID &&
		r.Name == s.Name &&
		r.Email == s.Email &&
		r.IsSkippingTomorrow == s.IsSkippingTomorrow &&
		maps.Equal(r.Schedule, s.Schedule) &&
		r.IsSubscribed == s.IsSubscribed &&
		r.CurrentlyAtRC == s.CurrentlyAtRC
}

func TestFirestoreReviewDB(t *testing.T) {
	t.Run("round-trip content", func(t *testing.T) {
		ctx := context.Background()
		projectID := fakeProjectID(t)

		client := testFirestoreClient(t, ctx, projectID)
		reviews := &FirestoreReviewDB{client}

		review := Review{
			Content:   "test review",
			Email:     "test@recurse.example.net",
			Timestamp: randInt64(t),
		}

		err := reviews.Insert(ctx, review)
		if err != nil {
			t.Fatal(err)
		}

		// Reviews are returned as a slice, even for just one review
		expected := []Review{review}

		actual, err := reviews.GetLastN(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}

		if len(actual) != len(expected) {
			t.Fatalf("number of reviews not equal:\nactual:   %d\nexpected: %d", len(actual), len(expected))
		}

		if !actual[0].Equal(expected[0]) {
			t.Errorf("values not equal:\nactual:   %+v\nexpected: %+v", actual[0], expected[0])
		}
	})
}

func (r Review) Equal(s Review) bool {
	return r.Content == s.Content &&
		r.Email == s.Email &&
		r.Timestamp == s.Timestamp
}

func TestFirestoreAuthDB(t *testing.T) {
	ctx := context.Background()
	projectID := fakeProjectID(t)

	client := testFirestoreClient(t, ctx, projectID)
	auth := &FirestoreAPIAuthDB{client}

	// Try to keep tests from conflicting with each other by adding a token
	// that only this test knows about.
	key := fmt.Sprintf("token-%d", randInt64(t))
	val := fmt.Sprintf("secret-%d", randInt64(t))
	doc := map[string]any{
		"value": val,
	}
	_, err := client.Collection("testing").Doc(key).Set(ctx, doc)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("missing", func(t *testing.T) {
		_, err := auth.GetToken(ctx, "does-not/exist")
		if status.Code(err) != codes.NotFound {
			t.Fatalf("expected NotFound error, got %#+v", err)
		}
	})

	t.Run("present", func(t *testing.T) {
		actual, err := auth.GetToken(ctx, fmt.Sprintf("testing/%s", key))
		if err != nil {
			t.Fatal(err)
		}

		if actual != val {
			t.Errorf("values not equal:\nactual:   %+v\nexpected: %+v", actual, val)
		}
	})
}

func TestFirestorePairingsDB(t *testing.T) {
	t.Run("round trip weekly pairings", func(t *testing.T) {
		ctx := context.Background()
		projectID := fakeProjectID(t)

		client := testFirestoreClient(t, ctx, projectID)
		pairings := &FirestorePairingsDB{client}

		// Entries representing pairings for each day of the week
		for i := 6; i >= 0; i-- {
			err := pairings.SetNumPairings(ctx, int(time.Now().Add(-time.Duration(i)*24*time.Hour).Unix()), 5)
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
