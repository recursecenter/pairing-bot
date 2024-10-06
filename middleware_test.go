package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/recursecenter/pairing-bot/internal/assert"
)

func Test_cron(t *testing.T) {
	t.Run("run job for AppEngine", func(t *testing.T) {
		// Arrange a cron job that tells us whether it ran.
		ran := false
		handler := cron(func(context.Context) error {
			ran = true
			return nil
		})

		// Prepare an AppEngine-sourced request.
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Appengine-Cron", "true")

		// Run it!
		w := httptest.NewRecorder()
		handler(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		assert.Equal(t, ran, true)
		assert.Equal(t, resp.StatusCode, 200)
	})

	t.Run("deny request outside of cron", func(t *testing.T) {
		// Arrange a cron job that fails the test if it runs.
		handler := cron(func(context.Context) error {
			t.Error("handler should not have run")
			return nil
		})

		// Prepare a request from outside of AppEngine (no custom header).
		req := httptest.NewRequest(http.MethodGet, "/", nil)

		// Run it!
		w := httptest.NewRecorder()
		handler(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		assert.Equal(t, resp.StatusCode, 404)
	})

	t.Run("report job failure", func(t *testing.T) {
		// Arrange a cron job that errors.
		handler := cron(func(context.Context) error {
			return errors.New("test error")
		})

		// Prepare an AppEngine-sourced request.
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Appengine-Cron", "true")

		// Run it!
		w := httptest.NewRecorder()
		handler(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		assert.Equal(t, resp.StatusCode, 500)
	})
}
