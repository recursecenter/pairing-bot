package zulip

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Webhook is the payload for an incoming Zulip webhook request.
//
// https://zulip.com/api/outgoing-webhooks#fields-documentation
type Webhook struct {
	Data    string `json:"data"`
	Token   string `json:"token"`
	Trigger string `json:"trigger"`

	Message Message `json:"message"`
}

// Message contains the details of the chat message that triggered the webhook.
//
// https://zulip.com/api/outgoing-webhooks#fields-documentation
type Message struct {
	DisplayRecipient DisplayRecipient `json:"display_recipient"`
	SenderID         int64            `json:"sender_id"`
	SenderEmail      string           `json:"sender_email"`
	SenderFullName   string           `json:"sender_full_name"`
}

// DisplayRecipient represents the recipient of the message, either a stream
// name or a set of users. Exactly one field (Stream xor Users) will be set.
//
// https://zulip.com/api/outgoing-webhooks#fields-documentation
type DisplayRecipient struct {
	Stream string
	Users  []User
}

func (d *DisplayRecipient) UnmarshalJSON(b []byte) error {
	// Ignore null the same way the stdlib does.
	if b == nil || bytes.Equal(b, []byte("null")) {
		return nil
	}

	// Check each

	var stream string
	if err := json.Unmarshal(b, &stream); err == nil {
		*d = DisplayRecipient{Stream: stream}
		return nil
	}

	var users []User
	if err := json.Unmarshal(b, &users); err == nil {
		*d = DisplayRecipient{Users: users}
		return nil
	}

	return errors.New("invalid value for DisplayRecipient")
}

type User struct {
	ID int64 `json:"id"`
}

// Response is the bot's response to the message tha triggered the webhook.
//
// If Content is intentionally empty, set ResponseNotRequired to true to
// indicate to Zulip that this is the expected.
//
// https://zulip.com/api/outgoing-webhooks#replying-with-a-message
type Response struct {
	Content             string `json:"content"`
	ResponseNotRequired bool   `json:"response_not_required"`
}

// NoResponse returns an intentionally empty Response.
func NoResponse() Response {
	return Response{ResponseNotRequired: true}
}

// Reply returns a Response containing a chat message to send.
func Reply(content string) Response {
	return Response{Content: content}
}

var ErrWebhookParse = errors.New("could not parse webhook")
var ErrWebhookUnauthorized = errors.New("unauthorized interaction attempt")

// ParseWebhook decodes a Webhook and validates that it came from Zulip by
// comparing against the shared secret token.
func ParseWebhook(r io.Reader, token string) (*Webhook, error) {
	w := new(Webhook)
	if err := json.NewDecoder(r).Decode(w); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrWebhookParse, err)
	}

	if w.Token != token {
		return nil, ErrWebhookUnauthorized
	}
	return w, nil
}
