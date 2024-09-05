package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"maps"
	"math/big"
	"strconv"
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

		// GetByUserID will prefer the argument values for email and name if
		// they differ from what's stored in the DB. These values are the same,
		// so we wouldn't be able to tell from this call.
		unchanged, err := recursers.GetByUserID(ctx, recurser.id, recurser.email, recurser.name)
		if err != nil {
			t.Fatal(err)
		}

		if !unchanged.Equal(expected) {
			t.Errorf("values not equal:\nactual:   %+v\nexpected: %+v", unchanged, expected)
		}

		// These values are different, so this call *does* tell us whether we
		// used the arguments.
		changed, err := recursers.GetByUserID(ctx, recurser.id, "changed@recurse.example.net", "My Name")
		if err != nil {
			t.Fatal(err)
		}

		expected.email = "changed@recurse.example.net"
		expected.name = "My Name"

		if !changed.Equal(expected) {
			t.Errorf("values not equal:\nactual:   %+v\nexpected: %+v", changed, expected)
		}

		// But none of this is actually stored in the DB. If we fetch the
		// collection directly, we can see the original name and email.
		doc, err := client.Collection("recursers").Doc(strconv.FormatInt(recurser.id, 10)).Get(ctx)
		if err != nil {
			t.Fatal(err)
		}

		actual, err := parseDoc(doc)
		if err != nil {
			t.Fatal(err)
		}

		// parseDoc forces this to `true`, so undo that.
		if !actual.isSubscribed {
			t.Error("isSubscribed should have been true, was false")
		}
		actual.isSubscribed = false

		expected = recurser
		expected.isSubscribed = true

		if !actual.Equal(recurser) {
			t.Errorf("values not equal:\nactual:   %+v\nexpected: %+v", actual, recurser)
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
