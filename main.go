package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/firestore"
)

// It's alive! The application starts here.
func main() {
	// Log the date and time (to the second),
	// in UTC regardles of local time zone,
	// and the file:line (without the full path- we don't have directories.)
	log.SetFlags(log.Ldate | log.Ltime | log.LUTC | log.Lshortfile)

	ctx := context.Background()

	appEnv := os.Getenv("APP_ENV")
	projectId := "pairing-bot-284823"
	botUsername := "pairing-bot@recurse.zulipchat.com"

	log.Printf("Running the app in environment = %s", appEnv)

	//We have two pairing bot projects. One for production and one for testing/dev work.
	if appEnv != "production" {
		projectId = "pairing-bot-dev"
		botUsername = "dev-pairing-bot@recurse.zulipchat.com"
		log.Println("Running pairing bot in the testing environment for development")
	}

	// Set up database wrappers. The Firestore client has a connection pool, so
	// we can share this one DB handle among all the collection helpers.
	db, err := firestore.NewClient(ctx, projectId)
	if err != nil {
		log.Panic(err)
	}
	defer db.Close()

	rdb := &FirestoreRecurserDB{
		client: db,
	}

	rcapi := RecurseAPI{
		rcAPIURL: "https://www.recurse.com/api/v1",
	}

	adb := &FirestoreAPIAuthDB{
		client: db,
	}

	pdb := &FirestorePairingsDB{
		client: db,
	}

	revdb := &FirestoreReviewDB{
		client: db,
	}

	ur := &zulipUserRequest{}

	un := &zulipUserNotification{
		botUsername: botUsername,
		zulipAPIURL: "https://recurse.zulipchat.com/api/v1/messages",
	}

	sm := &zulipStreamMessage{
		botUsername: botUsername,
		zulipAPIURL: "https://recurse.zulipchat.com/api/v1/messages",
	}

	pl := &PairingLogic{
		rdb:   rdb,
		adb:   adb,
		pdb:   pdb,
		ur:    ur,
		un:    un,
		sm:    sm,
		rcapi: rcapi,
		revdb: revdb,
	}

	http.HandleFunc("/", http.NotFound)           // will this handle anything that's not defined?
	http.HandleFunc("/webhooks", pl.handle)       // from zulip
	http.HandleFunc("/match", pl.match)           // from GCP- daily
	http.HandleFunc("/endofbatch", pl.endofbatch) // from GCP- weekly
	http.HandleFunc("/welcome", pl.welcome)       // from GCP- weekly
	http.HandleFunc("/checkin", pl.checkin)       // from GCP- weekly

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Defaulting to port %s", port)
	}

	if m, ok := os.LookupEnv("PB_MAINT"); ok {
		if m == "true" {
			maintenanceMode = true
		}
	}

	log.Printf("Listening on port %s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}
