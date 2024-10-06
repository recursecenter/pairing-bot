package main

import (
	"context"
	"log/slog"
	"net/http"
)

// JobFunc is the type of function that can run as a cron job.
type JobFunc func(context.Context) error

// cron wraps a job function to make it an HTTP handler. The handler enforces
// that requests originate from App Engine's Cron scheduler.
func cron(job JobFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check that the request is originating from within app engine
		// https://cloud.google.com/appengine/docs/standard/go/scheduling-jobs-with-cron-yaml#validating_cron_requests
		if r.Header.Get("X-Appengine-Cron") != "true" {
			http.NotFound(w, r)
			return
		}

		err := job(r.Context())
		if err != nil {
			slog.Error("Job failed", slog.Any("error", err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}
