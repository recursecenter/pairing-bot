package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

// It's alive! The application starts here.
func main() {
	http.HandleFunc("/", nope)
	http.HandleFunc("/webhooks", handle)
	http.HandleFunc("/match", match)
	http.HandleFunc("/endofbatch", endofbatch)

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
