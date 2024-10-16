package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"

	"cloud.google.com/go/firestore"
	"github.com/recursecenter/pairing-bot/recurse"
	"github.com/recursecenter/pairing-bot/store"
	"github.com/recursecenter/pairing-bot/zulip"
)

// It's alive! The application starts here.
func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		AddSource: true,
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			// Leave all nested attributes as-is.
			if len(groups) != 0 {
				return attr
			}

			// Rewrite some standard fields to match the logging agent's
			// expectations.
			//
			// https://cloud.google.com/logging/docs/agent/logging/configuration#special-fields
			switch attr.Key {
			case slog.MessageKey:
				attr.Key = "message"
			case slog.LevelKey:
				attr.Key = "severity"
			case slog.SourceKey:
				attr.Key = "logging.googleapis.com/sourceLocation"
			}
			return attr
		},
	})))

	ctx := context.Background()

	appVersion := os.Getenv("GAE_VERSION")

	appEnv := os.Getenv("APP_ENV")

	projectId := "pairing-bot-284823"
	botUsername := "pairing-bot@recurse.zulipchat.com"
	welcomeStream := "current batches"

	log.Printf("Running the app in environment = %s", appEnv)

	// We have two pairing bot projects. One for production and one for testing/dev work.
	if appEnv != "production" {
		slog.Info("Setting dev/test config values")

		projectId = "pairing-bot-dev"
		botUsername = "dev-pairing-bot@recurse.zulipchat.com"
		welcomeStream = "test-bot"
	}

	// Set up database wrappers. The Firestore client has a connection pool, so
	// we can share this one DB handle among all the collection helpers.
	db, err := firestore.NewClient(ctx, projectId)
	if err != nil {
		log.Panic(err)
	}
	defer db.Close()

	zulipCredentials := func(ctx context.Context) (zulip.Credentials, error) {
		password, err := store.Secrets(db).Get(ctx, "zulip_api_key")
		if err != nil {
			return zulip.Credentials{}, err
		}

		return zulip.Credentials{
			Username: botUsername,
			Password: password,
		}, nil
	}

	zulipClient, err := zulip.NewClient(zulipCredentials)
	if err != nil {
		panic(err)
	}

	recurseAccessToken := func(ctx context.Context) (recurse.AccessToken, error) {
		token, err := store.Secrets(db).Get(ctx, "recurse_access_token")
		if err != nil {
			return "", err
		}
		return recurse.AccessToken(token), nil
	}

	recurseClient, err := recurse.NewClient(recurseAccessToken)
	if err != nil {
		panic(err)
	}

	pl := &PairingLogic{
		db:      db,
		recurse: recurseClient,
		zulip:   zulipClient,

		version:       appVersion,
		welcomeStream: welcomeStream,
	}

	http.HandleFunc("/", http.NotFound)                 // will this handle anything that's not defined?
	http.HandleFunc("/webhooks", pl.handle)             // from zulip
	http.HandleFunc("/match", cron(pl.Match))           // from GCP- daily
	http.HandleFunc("/endofbatch", cron(pl.EndOfBatch)) // from GCP- weekly
	http.HandleFunc("/welcome", cron(pl.Welcome))       // from GCP- weekly
	http.HandleFunc("/checkin", cron(pl.Checkin))       // from GCP- weekly

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Defaulting to port %s", port)
	}

	if m, ok := os.LookupEnv("PB_MAINT"); ok {
		if m == "true" {
			pl.maintenanceMode = true
		}
	}

	log.Printf("Listening on port %s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}
