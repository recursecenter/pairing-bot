package store

import (
	"context"
	"math/rand"

	"cloud.google.com/go/firestore"
)

type Review struct {
	Content   string `firestore:"content"`
	Email     string `firestore:"email"`
	Timestamp int64  `firestore:"timestamp"`
}

// ReviewsClient manages user-submitted Pairing Bot reviews.
type ReviewsClient struct {
	client *firestore.Client
}

func Reviews(client *firestore.Client) *ReviewsClient {
	return &ReviewsClient{client}
}

func (r *ReviewsClient) GetAll(ctx context.Context) ([]Review, error) {
	iter := r.client.Collection("reviews").Documents(ctx)
	return fetchAll[Review](iter)
}

func (r *ReviewsClient) GetLastN(ctx context.Context, n int) ([]Review, error) {
	iter := r.client.
		Collection("reviews").
		OrderBy("timestamp", firestore.Desc).
		Limit(n).
		Documents(ctx)
	return fetchAll[Review](iter)
}

func (r *ReviewsClient) GetRandom(ctx context.Context) (Review, error) {
	allReviews, err := r.GetAll(ctx)

	if err != nil {
		return Review{}, err
	}

	return allReviews[rand.Intn(len(allReviews))], nil
}

func (r *ReviewsClient) Insert(ctx context.Context, review Review) error {
	_, _, err := r.client.Collection("reviews").Add(ctx, review)
	return err
}
