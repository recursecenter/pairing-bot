package zulip_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/recursecenter/pairing-bot/zulip"
)

func assertEqual[T any](t *testing.T, expected, actual T) {
	t.Helper()

	if reflect.DeepEqual(expected, actual) {
		return
	}

	t.Errorf("expected %#+v, got %#+v", expected, actual)
}

func assertNoError(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		return
	}

	t.Errorf("expected no error, got %#+v", err)
}

func assertErrorAs[T error](t *testing.T, err error) (target T, ok bool) {
	ok = errors.As(err, &target)
	if !ok {
		t.Errorf("expected error as %T, got %#+v", target, err)
	}
	return target, ok
}

func withEnv(t *testing.T, name string, value string) {
	prev, wasSet := os.LookupEnv(name)
	if err := os.Setenv(name, value); err != nil {
		t.Fatalf("set env %q: %s", name, err)
	}

	if wasSet {
		t.Cleanup(func() {
			if err := os.Setenv(name, prev); err != nil {
				panic(err)
			}
		})
	}
}

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

	assertEqual(m.t, int64(expected), m.requestCount.Load())
}

func TestClient_PostToTopic(t *testing.T) {
	t.Run("non-production", func(t *testing.T) {
		withEnv(t, "APP_ENV", "development")

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
		withEnv(t, "APP_ENV", "production")

		srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
			assertEqual(t, r.Method, http.MethodPost)
			assertEqual(t, r.URL.Path, "/messages")

			// Base64-encoding of "fake-username:fake-password"
			authz := "Basic ZmFrZS11c2VybmFtZTpmYWtlLXBhc3N3b3Jk"
			assertEqual(t, r.Header.Get("Authorization"), authz)

			assertEqual(t, r.Header.Get("Content-Type"), "application/x-www-form-urlencoded")

			form := url.Values{
				"type":    []string{"stream"},
				"to":      []string{"checkouts"},
				"topic":   []string{"Pearing Bot"},
				"content": []string{"Later, y'all!"},
			}
			assertNoError(t, r.ParseForm())
			assertEqual(t, form, r.Form)
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
		assertEqual(t, r.Method, http.MethodPost)
		assertEqual(t, r.URL.Path, "/messages")

		// Base64-encoding of "fake-username:fake-password"
		authz := "Basic ZmFrZS11c2VybmFtZTpmYWtlLXBhc3N3b3Jk"
		assertEqual(t, r.Header.Get("Authorization"), authz)

		assertEqual(t, r.Header.Get("Content-Type"), "application/x-www-form-urlencoded")

		form := url.Values{
			"type":    []string{"private"},
			"to":      []string{"[0,1]"},
			"content": []string{"Okay, go!"},
		}
		assertNoError(t, r.ParseForm())
		assertEqual(t, form, r.Form)
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
	assertEqual(t, "error response from Zulip: 418 I'm a teapot", err.Error())

	if respErr, ok := assertErrorAs[*zulip.ResponseError](t, err); ok {
		assertEqual(t, 418, respErr.Response.StatusCode)

		body, readErr := io.ReadAll(respErr.Response.Body)
		assertNoError(t, readErr)
		assertEqual(t, "oops!", string(body))
	}

	srv.AssertRequestCount(1)
}
