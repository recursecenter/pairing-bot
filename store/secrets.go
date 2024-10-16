package store

import (
	"context"

	"cloud.google.com/go/firestore"
)

// SecretsClient manages auth tokens stored in Firestore.
type SecretsClient struct {
	client *firestore.Client
}

func Secrets(client *firestore.Client) *SecretsClient {
	return &SecretsClient{client}
}

func (s *SecretsClient) Get(ctx context.Context, name string) (string, error) {
	doc, err := s.client.Collection("secrets").Doc(name).Get(ctx)
	if err != nil {
		return "", err
	}

	var token struct {
		Value string `firestore:"value"`
	}

	if err := doc.DataTo(&token); err != nil {
		return "", err
	}

	return token.Value, nil
}
