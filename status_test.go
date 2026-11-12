package main

import (
	"context"
	"testing"

	"github.com/recursecenter/pairing-bot/store"
)

func TestStatus(t *testing.T) {
	ctx := context.Background()
	pl := &PairingLogic{}

	tests := []struct {
		name     string
		schedule map[string]bool
		skipping bool
		want     string
	}{
		{
			name:     "empty schedule",
			schedule: store.EmptySchedule(),
			want:     "* You're Your Name\n* You're not scheduled for pairing on any days\n* **You're not set to skip** pairing tomorrow",
		},
		{
			name:     "empty schedule, skipping tomorrow",
			schedule: store.EmptySchedule(),
			skipping: true,
			want:     "* You're Your Name\n* You're not scheduled for pairing on any days\n* **You're set to skip** pairing tomorrow",
		},
		{
			name:     "nil schedule",
			schedule: nil,
			want:     "* You're Your Name\n* You're not scheduled for pairing on any days\n* **You're not set to skip** pairing tomorrow",
		},
		{
			name:     "one day",
			schedule: store.NewSchedule([]string{"monday"}),
			want:     "* You're Your Name\n* You're scheduled for pairing on **Mondays**\n* **You're not set to skip** pairing tomorrow",
		},
		{
			name:     "three days",
			schedule: store.NewSchedule([]string{"friday", "monday", "wednesday"}),
			want:     "* You're Your Name\n* You're scheduled for pairing on **Mondays, Wednesdays, and Fridays**\n* **You're not set to skip** pairing tomorrow",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &store.Recurser{
				Name:               "Your Name",
				Schedule:           tt.schedule,
				IsSkippingTomorrow: tt.skipping,
				IsSubscribed:       true,
			}

			got, err := pl.Status(ctx, rec)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
