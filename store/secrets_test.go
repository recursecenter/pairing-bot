package store_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/recursecenter/pairing-bot/internal/pbtest"
	"github.com/recursecenter/pairing-bot/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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
