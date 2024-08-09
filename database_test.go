package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"maps"
	"math/big"
	"testing"

	"cloud.google.com/go/firestore"
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
			id:                 randInt64(t),
			name:               "Your Name",
			email:              "test@recurse.example.net",
			isSkippingTomorrow: false,
			schedule: map[string]any{
				"monday":    false,
				"tuesday":   false,
				"wednesday": false,
				"thursday":  false,
				"friday":    false,
				"saturday":  false,
				"sunday":    false,
			},
			isSubscribed:  false,
			currentlyAtRC: false,
		}

		err := recursers.Set(ctx, recurser.id, recurser)
		if err != nil {
			t.Fatal(err)
		}

		// GetByUserID forces isSubscribed to be `true`, because that's implied by
		// the record's existence in the DB in the first place.
		expected := recurser
		expected.isSubscribed = true

		// GetByUserID can update the name and email address if our record is stale.
		// These values are the same, so this call *does not* trigger that update.
		unchanged, err := recursers.GetByUserID(ctx, recurser.id, recurser.email, recurser.name)
		if err != nil {
			t.Fatal(err)
		}

		if !unchanged.Equal(expected) {
			t.Errorf("values not equal:\nactual:   %+v\nexpected: %+v", unchanged, expected)
		}

		// These values are different, so this call *does* trigger an update.
		changed, err := recursers.GetByUserID(ctx, recurser.id, "changed@recurse.example.net", "My Name")
		if err != nil {
			t.Fatal(err)
		}

		expected.email = "changed@recurse.example.net"
		expected.name = "My Name"

		if !changed.Equal(expected) {
			t.Errorf("values not equal:\nactual:   %+v\nexpected: %+v", unchanged, expected)
		}
	})
}

func (r Recurser) Equal(s Recurser) bool {
	return r.id == s.id &&
		r.name == s.name &&
		r.email == s.email &&
		r.isSkippingTomorrow == s.isSkippingTomorrow &&
		maps.Equal(r.schedule, s.schedule) &&
		r.isSubscribed == s.isSubscribed &&
		r.currentlyAtRC == s.currentlyAtRC
}
