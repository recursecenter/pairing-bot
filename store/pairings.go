package store

import (
	"context"
	"log"
	"strconv"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
)

type Pairing struct {
	Value     int   `firestore:"value"`
	Timestamp int64 `firestore:"timestamp"`
}

// PairingsClient manages pairing (matching) result records.
type PairingsClient struct {
	client *firestore.Client
}

func Pairings(client *firestore.Client) *PairingsClient {
	return &PairingsClient{client}
}

func (p *PairingsClient) SetNumPairings(ctx context.Context, pairing Pairing) error {
	timestampAsString := strconv.FormatInt(pairing.Timestamp, 10)

	_, err := p.client.Collection("pairings").Doc(timestampAsString).Set(ctx, pairing)
	return err
}

func (p *PairingsClient) GetTotalPairingsDuringLastWeek(ctx context.Context) (int, error) {
	totalPairings := 0

	timestampSevenDaysAgo := time.Now().Add(-7 * 24 * time.Hour).Unix()

	iter := p.client.Collection("pairings").Where("timestamp", ">", timestampSevenDaysAgo).Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return 0, err
		}

		var pairing Pairing
		if err = doc.DataTo(&pairing); err != nil {
			log.Printf("Skipping %q: %s", doc.Ref.Path, err)
			continue
		}

		log.Println("The timestamp is: ", pairing.Timestamp)

		totalPairings += pairing.Value
	}

	return totalPairings, nil
}
