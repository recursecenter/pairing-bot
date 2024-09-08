package recurse_test

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/recursecenter/pairing-bot/recurse"
)

func assertEqual[T any](t *testing.T, expected, actual T) {
	t.Helper()

	if reflect.DeepEqual(expected, actual) {
		return
	}

	t.Errorf("expected %#+v to equal %#+v", expected, actual)
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
	m.Close()

	assertEqual(m.t, int64(expected), m.requestCount.Load())
}

func (m *MockServer) Close() {
	m.srv.Close()
}

func fakeProfiles(t *testing.T, n int) []recurse.Profile {
	var all []recurse.Profile

	for i := range n {
		all = append(all, recurse.Profile{
			Name:    fmt.Sprintf("Name %d", i),
			ZulipID: int64(i),
		})
	}

	return all
}

func min[T cmp.Ordered](a T, rest ...T) T {
	for _, b := range rest {
		if b < a {
			a = b
		}
	}
	return a
}

func TestClient_ActiveRecursers(t *testing.T) {
	type PageParams struct {
		Offset int
		Limit  int
	}

	for count, expectedPages := range map[int][]PageParams{
		0: {
			{0, 50}, // Empty page
		},
		1: {
			{0, 50}, // Partial page
		},
		50: {
			{0, 50},  // Full page
			{50, 50}, // Empty page
		},
		51: {
			{0, 50},  // Full page
			{50, 50}, // Partial page
		},
		100: {
			{0, 50},   // Full page
			{50, 50},  // Full page
			{100, 50}, // Empty page
		},
	} {
		t.Run(fmt.Sprintf("%d recursers", count), func(t *testing.T) {
			allProfiles := fakeProfiles(t, count)

			pageIdx := 0
			srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
				assertEqual(t, http.MethodGet, r.Method)
				assertEqual(t, "/profiles", r.URL.Path)

				page := expectedPages[pageIdx]
				pageIdx++

				params := url.Values{
					"access_token": []string{"fake-access-token"},
					"scope":        []string{"current"},
					"role":         []string{"recurser"},
					"offset":       []string{strconv.Itoa(page.Offset)},
					"limit":        []string{strconv.Itoa(page.Limit)},
				}
				assertEqual(t, params, r.URL.Query())

				start := page.Offset
				end := min(start+page.Limit, len(allProfiles))

				err := json.NewEncoder(w).Encode(allProfiles[start:end])
				if err != nil {
					t.Fatal(err)
				}
			})
			defer srv.AssertRequestCount(len(expectedPages))

			client, err := recurse.NewClient(
				recurse.StaticAccessToken("fake-access-token"),
				recurse.WithHTTP(srv.Client()),
				recurse.WithBaseURL(srv.URL()),
			)
			if err != nil {
				t.Fatal(err)
			}

			ctx := context.Background()

			profiles, err := client.ActiveRecursers(ctx)
			if err != nil {
				t.Fatal(err)
			}

			assertEqual(t, profiles, allProfiles)
		})
	}
}

func loadJSON[T any](t *testing.T, path string) T {
	t.Helper()

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestClient_AllBatches(t *testing.T) {
	allBatches := loadJSON[[]recurse.Batch](t, "testdata/batches.json")

	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertEqual(t, http.MethodGet, r.Method)
		assertEqual(t, "/batches", r.URL.Path)

		params := url.Values{
			"access_token": []string{"fake-access-token"},
		}
		assertEqual(t, params, r.URL.Query())

		err := json.NewEncoder(w).Encode(allBatches)
		if err != nil {
			t.Fatal(err)
		}
	})
	defer srv.AssertRequestCount(1)

	client, err := recurse.NewClient(
		recurse.StaticAccessToken("fake-access-token"),
		recurse.WithHTTP(srv.Client()),
		recurse.WithBaseURL(srv.URL()),
	)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	batches, err := client.AllBatches(ctx)
	if err != nil {
		t.Fatal(err)
	}

	assertEqual(t, batches, allBatches)
}

func TestClient_IsCurrentlyAtRC(t *testing.T) {
	// This handler is easier than the one for ActiveRecursers because we know
	// there's exactly one page to return.
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertEqual(t, http.MethodGet, r.Method)
		assertEqual(t, "/profiles", r.URL.Path)

		params := url.Values{
			"access_token": []string{"fake-access-token"},
			"scope":        []string{"current"},
			"role":         []string{"recurser"},
			"offset":       []string{"0"},
			"limit":        []string{"50"},
		}
		assertEqual(t, params, r.URL.Query())

		page := fakeProfiles(t, 25)

		err := json.NewEncoder(w).Encode(page)
		if err != nil {
			t.Fatal(err)
		}
	})
	defer srv.Close()

	client, err := recurse.NewClient(
		recurse.StaticAccessToken("fake-access-token"),
		recurse.WithHTTP(srv.Client()),
		recurse.WithBaseURL(srv.URL()),
	)
	if err != nil {
		t.Fatal(err)
	}

	for zulipID, expected := range map[int64]bool{
		0:    true,
		5:    true,
		24:   true,
		25:   false,
		1000: false,
	} {
		t.Run(fmt.Sprintf("%d", zulipID), func(t *testing.T) {
			ctx := context.Background()

			atRC, err := client.IsCurrentlyAtRC(ctx, zulipID)
			if err != nil {
				t.Fatal(err)
			}

			assertEqual(t, atRC, expected)
		})
	}
}

func TestClient_recurse_errors(t *testing.T) {
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		if _, err := w.Write([]byte("oops!")); err != nil {
			panic(err)
		}
	})

	client, err := recurse.NewClient(
		recurse.StaticAccessToken("fake-access-token"),
		recurse.WithHTTP(srv.Client()),
		recurse.WithBaseURL(srv.URL()),
	)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	_, err = client.ActiveRecursers(ctx)
	assertEqual(t, "get active recursers (offset=0): error response from Recurse: 418 I'm a teapot", err.Error())

	if respErr, ok := assertErrorAs[*recurse.ResponseError](t, err); ok {
		assertEqual(t, 418, respErr.Response.StatusCode)

		body, readErr := io.ReadAll(respErr.Response.Body)
		assertNoError(t, readErr)
		assertEqual(t, "oops!", string(body))
	}

	srv.AssertRequestCount(1)
}

func TestBatch_IsMini(t *testing.T) {
	batches := loadJSON[[]recurse.Batch](t, "testdata/batches.json")

	t.Run("mini batch", func(t *testing.T) {
		batch := batches[2]
		assertEqual(t, true, batch.IsMini())
	})

	t.Run("not-mini batch", func(t *testing.T) {
		batch := batches[0]
		assertEqual(t, false, batch.IsMini())
	})
}

func mustJSON[T any](t *testing.T, data string) T {
	t.Helper()

	var v T
	if err := json.Unmarshal([]byte(data), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func TestBatch_IsSecondWeek(t *testing.T) {
	batch := mustJSON[recurse.Batch](t, `
	  {
	    "id": 166,
	    "name": "Summer 1, 2024",
	    "start_date": "2024-05-20",
	    "end_date": "2024-08-09"
	  }
	`)

	week1cron := must(time.Parse(time.RFC3339, "2024-05-21T14:00:00-04:00"))
	week2cron := must(time.Parse(time.RFC3339, "2024-05-28T14:00:00-04:00"))
	week3cron := must(time.Parse(time.RFC3339, "2024-06-04T14:00:00-04:00"))

	assertEqual(t, false, batch.IsSecondWeek(week1cron))
	assertEqual(t, true, batch.IsSecondWeek(week2cron))
	assertEqual(t, false, batch.IsSecondWeek(week3cron))
}
