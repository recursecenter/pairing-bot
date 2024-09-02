package zulip_test

import (
	"bytes"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/recursecenter/pairing-bot/zulip"
)

func TestWebhook(t *testing.T) {
	goodHooks := map[string]zulip.Webhook{
		"testdata/webhook_direct_message.json": {
			Data:    "schedule mon wed friday",
			Token:   "fake-zulip-token",
			Trigger: "direct_message",
			Message: zulip.Message{
				DisplayRecipient: zulip.DisplayRecipient{
					Users: []zulip.User{
						{ID: 1000},
						{ID: 2000},
					},
				},
				SenderID:       1000,
				SenderEmail:    "fake-1000@recurse.example.net",
				SenderFullName: "Your Name",
			},
		},
		"testdata/webhook_mention.json": {
			Data:    "Try messaging @**Pairing Bot!** to join in!",
			Token:   "fake-zulip-token",
			Trigger: "mention",
			Message: zulip.Message{
				DisplayRecipient: zulip.DisplayRecipient{Stream: "Stream Channel"},
				SenderID:         1000,
				SenderEmail:      "fake-1000@recurse.example.net",
				SenderFullName:   "Your Name",
			},
		},
	}

	badHooks := map[string]error{
		"testdata/webhook_bad_data.json":  zulip.ErrWebhookParse,
		"testdata/webhook_bad_token.json": zulip.ErrWebhookUnauthorized,
	}

	for path, expected := range goodHooks {
		t.Run(path, func(t *testing.T) {
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			w, err := zulip.ParseWebhook(bytes.NewReader(b), "fake-zulip-token")
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(*w, expected) {
				t.Errorf("expected %+v, got %+v", expected, *w)
			}
		})
	}

	for path, expected := range badHooks {
		t.Run(path, func(t *testing.T) {
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			_, err = zulip.ParseWebhook(bytes.NewReader(b), "fake-zulip-token")
			if !errors.Is(err, expected) {
				t.Errorf("expected %v, got %v", expected, err)
			}
		})
	}
}
