package zulip_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/recursecenter/pairing-bot/internal/assert"
	"github.com/recursecenter/pairing-bot/zulip"
)

// MockServer wraps an httptest.Server to count the number of requests
// received.
type MockServer struct {
	t *testing.T

	srv          *httptest.Server
	requestCount *atomic.Int64
}

func mockServer(t *testing.T, handle http.HandlerFunc) *MockServer {
	mock := &MockServer{
		t: t,

		requestCount: new(atomic.Int64),
	}
	mock.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mock.requestCount.Add(1)
		handle.ServeHTTP(w, r)
	}))
	return mock
}

func (m *MockServer) Client() *http.Client {
	return m.srv.Client()
}

func (m *MockServer) URL() string {
	return m.srv.URL
}

func (m *MockServer) AssertRequestCount(expected int) {
	m.t.Helper()
	m.srv.Close()

	assert.Equal(m.t, m.requestCount.Load(), int64(expected))
}

func TestClient_PostToTopic(t *testing.T) {
	t.Run("non-production", func(t *testing.T) {
		t.Setenv("APP_ENV", "development")

		srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
			t.Fatalf("unexpected request: %#+v", r)
		})

		client, err := zulip.NewClient(
			zulip.StaticCredentials("fake-username", "fake-password"),
			zulip.WithHTTP(srv.Client()),
			zulip.WithBaseURL(srv.URL()),
		)
		if err != nil {
			t.Fatal(err)
		}

		ctx := context.Background()

		err = client.PostToTopic(ctx, "checkouts", "Pearing Bot", "Later, y'all!")
		if err != nil {
			t.Fatal(err)
		}

		srv.AssertRequestCount(0)
	})

	t.Run("production", func(t *testing.T) {
		t.Setenv("APP_ENV", "production")

		srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "/messages", r.URL.Path)

			// Base64-encoding of "fake-username:fake-password"
			authz := "Basic ZmFrZS11c2VybmFtZTpmYWtlLXBhc3N3b3Jk"
			assert.Equal(t, r.Header.Get("Authorization"), authz)

			assert.Equal(t, r.Header.Get("Content-Type"), "application/x-www-form-urlencoded")

			form := url.Values{
				"type":    []string{"stream"},
				"to":      []string{"checkouts"},
				"topic":   []string{"Pearing Bot"},
				"content": []string{"Later, y'all!"},
			}
			if assert.NoError(t, r.ParseForm()) {
				assert.Equal(t, r.Form, form)
			}
		})

		client, err := zulip.NewClient(
			zulip.StaticCredentials("fake-username", "fake-password"),
			zulip.WithHTTP(srv.Client()),
			zulip.WithBaseURL(srv.URL()),
		)
		if err != nil {
			t.Fatal(err)
		}

		ctx := context.Background()

		err = client.PostToTopic(ctx, "checkouts", "Pearing Bot", "Later, y'all!")
		if err != nil {
			t.Fatal(err)
		}

		srv.AssertRequestCount(1)
	})
}

func TestClient_SendUserMessage(t *testing.T) {
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/messages", r.URL.Path)

		// Base64-encoding of "fake-username:fake-password"
		authz := "Basic ZmFrZS11c2VybmFtZTpmYWtlLXBhc3N3b3Jk"
		assert.Equal(t, r.Header.Get("Authorization"), authz)

		assert.Equal(t, r.Header.Get("Content-Type"), "application/x-www-form-urlencoded")

		form := url.Values{
			"type":    []string{"private"},
			"to":      []string{"[0,1]"},
			"content": []string{"Okay, go!"},
		}
		if assert.NoError(t, r.ParseForm()) {
			assert.Equal(t, r.Form, form)
		}
	})

	client, err := zulip.NewClient(
		zulip.StaticCredentials("fake-username", "fake-password"),
		zulip.WithHTTP(srv.Client()),
		zulip.WithBaseURL(srv.URL()),
	)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	err = client.SendUserMessage(ctx, []int64{0, 1}, "Okay, go!")
	if err != nil {
		t.Fatal(err)
	}

	srv.AssertRequestCount(1)
}

func TestClient_zulip_errors(t *testing.T) {
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		if _, err := w.Write([]byte("oops!")); err != nil {
			panic(err)
		}
	})

	client, err := zulip.NewClient(
		zulip.StaticCredentials("fake-username", "fake-password"),
		zulip.WithHTTP(srv.Client()),
		zulip.WithBaseURL(srv.URL()),
	)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	err = client.SendUserMessage(ctx, []int64{0, 1}, "Okay, go!")
	assert.Equal(t, err.Error(), "error response from Zulip: 418 I'm a teapot")

	if respErr, ok := assert.ErrorAs[*zulip.ResponseError](t, err); ok {
		assert.Equal(t, respErr.Response.StatusCode, 418)

		body, readErr := io.ReadAll(respErr.Response.Body)
		if readErr != nil {
			t.Fatal(readErr)
		}

		assert.Equal(t, string(body), "oops!")
	}

	srv.AssertRequestCount(1)
}
