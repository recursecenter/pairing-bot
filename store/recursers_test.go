package store_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/recursecenter/pairing-bot/internal/assert"
	"github.com/recursecenter/pairing-bot/internal/pbtest"
	"github.com/recursecenter/pairing-bot/store"
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

