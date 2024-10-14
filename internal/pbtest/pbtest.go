// Package pbtest provides helpers for setting up Pairing Bot tests.
package pbtest

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"testing"

	"cloud.google.com/go/firestore"
)

// FirestoreClient returns a Firestore client scoped to a new random project ID.
func FirestoreClient(t *testing.T, ctx context.Context) *firestore.Client {
	client, err := firestore.NewClient(ctx, projectID(t))
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

// projectID generates a fake Google Cloud project ID for use in tests.
func projectID(t *testing.T) string {
	return fmt.Sprintf("fake-project-%d", RandInt64(t))
}

// RandInt64 generates a random number from the default source.
func RandInt64(t *testing.T) int64 {
	int64Max := int64(1<<63 - 1)

	n, err := rand.Int(rand.Reader, big.NewInt(int64Max))
	if err != nil {
		t.Fatal(err)
	}
	return n.Int64()
}
