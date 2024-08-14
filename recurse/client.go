package recurse

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var defaultBaseURL *url.URL = must(url.Parse("https://www.recurse.com/api/v1"))

// An AccessToken is used to authenticate requests to the Recurse API.
type AccessToken string

// AccessTokenFunc returns the current Recurse API access token.
type AccessTokenFunc func(context.Context) (AccessToken, error)

// A Client sends HTTP requests to the Recurse API.
type Client struct {
	http        *http.Client
	baseURL     *url.URL
	accessToken AccessTokenFunc
}

// NewClient creates a new Recurse API client.
func NewClient(accessToken AccessTokenFunc, opts ...ClientOpt) (*Client, error) {
	client := Client{
		http:        http.DefaultClient,
		baseURL:     defaultBaseURL,
		accessToken: accessToken,
	}

	for i, opt := range opts {
		if err := opt(&client); err != nil {
			return nil, fmt.Errorf("recurse client option %d: %w", i, err)
		}
	}

	return &client, nil
}

// A Profile contains information about a Recurser.
//
// Profile data is updated at midnight on the last day (Friday) of a batch.
//
// https://github.com/recursecenter/wiki/wiki/Recurse-Center-API#Profiles
type Profile struct {
	Name    string `json:"name"`
	ZulipID int64  `json:"zulip_id"`
}

// ActiveRecursers fetches the profiles for all recursers currently at RC.
//
// https://github.com/recursecenter/wiki/wiki/Recurse-Center-API#search
func (c *Client) ActiveRecursers(ctx context.Context) ([]Profile, error) {
	var profiles []Profile
	offset := 0
	limit := 50
	hasMore := true

	for hasMore {
		next, err := c.activeRecursers(ctx, offset, limit)
		if err != nil {
			return nil, fmt.Errorf("get active recursers (offset=%d): %w", offset, err)
		}

		// Move the offset cursor up by the number of profiles we got.
		offset += len(next)
		profiles = append(profiles, next...)

		// There may be more to read if we filled the current page.
		// If we didn't fill it, there's definitely nothing left.
		hasMore = len(next) == limit
	}

	return profiles, nil
}

// activeRecursers loads one page of profiles for recursers currently at RC.
//
// https://github.com/recursecenter/wiki/wiki/Recurse-Center-API#search
func (c *Client) activeRecursers(ctx context.Context, offset int, limit int) ([]Profile, error) {
	params := make(url.Values)
	params.Set("scope", "current")
	params.Set("offset", strconv.Itoa(offset))
	params.Set("limit", strconv.Itoa(limit))
	params.Set("role", "recurser")

	resp, err := c.get(ctx, "profiles", params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var profiles []Profile
	return profiles, json.NewDecoder(resp.Body).Decode(&profiles)
}

// Datestamp is a time.Time wrapper for parsing dates. It implements
// json.Unmarshaler by parsing the value with time.DateOnly in UTC.
type Datestamp time.Time

func (d *Datestamp) UnmarshalJSON(b []byte) error {
	// Ignore null the same way the stdlib does.
	if b == nil || bytes.Equal(b, []byte("null")) {
		return nil
	}

	// This is encoded as a JSON string, so unmarshal that first.
	var value string
	if err := json.Unmarshal(b, &value); err != nil {
		return err
	}

	// And now parse the string *contents* as a time value.
	t, err := time.ParseInLocation(time.DateOnly, value, time.UTC)
	if err != nil {
		return err
	}

	*d = Datestamp(t)
	return nil
}

func (d *Datestamp) MarshalJSON() ([]byte, error) {
	t := (*time.Time)(d)

	// Format it as a date string and then JSON-encode *that*.
	s := t.Format(time.DateOnly)
	return json.Marshal(s)
}

// A Batch is a cycle of the Recurse Center retreat.
//
// https://github.com/recursecenter/wiki/wiki/Recurse-Center-API#Batches
type Batch struct {
	Name      string    `json:"name"`
	StartDate Datestamp `json:"start_date"`
}

// IsMini returns whether the batch was a mini batch.
func (b Batch) IsMini() bool {
	return strings.HasPrefix(b.Name, "Mini")
}

// IsSecondWeek returns whether the time is within the batch's second week.
func (b Batch) IsSecondWeek(now time.Time) bool {
	activeTime := now.Sub(time.Time(b.StartDate))
	week := 7 * 24 * time.Hour
	return 1*week < activeTime && activeTime < 2*week
}

// AllBatches returns all RC batches up to the current batch with the most
// recent batch first.
//
// https://github.com/recursecenter/wiki/wiki/Recurse-Center-API#list-all-batches
func (c *Client) AllBatches(ctx context.Context) ([]Batch, error) {
	resp, err := c.get(ctx, "batches", nil)
	if err != nil {
		return nil, fmt.Errorf("get Recurse batches: %w", err)
	}
	defer resp.Body.Close()

	var batches []Batch
	return batches, json.NewDecoder(resp.Body).Decode(&batches)
}

// IsCurrentlyAtRC returns whether the user's Zulip ID appears in the list of
// profiles for recursers currently at RC.
func (c *Client) IsCurrentlyAtRC(ctx context.Context, zulipID int64) (bool, error) {
	// In practice, there aren't more than a few pages of Recursers currently
	// at RC. To save us from another pagination cursor, we can load everyone
	// at once and then scan for the Zulip ID.
	//
	// TODO(@jdkaplan): Query by RC ID instead to avoid iterating at all
	active, err := c.ActiveRecursers(ctx)
	if err != nil {
		return false, err
	}

	for _, profile := range active {
		if profile.ZulipID == zulipID {
			return true, nil
		}
	}

	return false, nil
}

// get sends the POST request with authorization and encoded query params. This
// returns a non-nil error if the response status code indicates an error (400
// or higher) or if the request could not be sent.
func (c *Client) get(ctx context.Context, path string, params url.Values) (*http.Response, error) {
	accessToken, err := c.accessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch credentials: %w", err)
	}

	if params == nil {
		params = make(url.Values)
	} else {
		params = maps.Clone(params)
	}
	params.Set("access_token", string(accessToken))

	url := c.baseURL.JoinPath(path)
	url.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		url.String(),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, &ResponseError{resp}
	}
	return resp, nil
}

// A ClientOpt is used to configure a Client.
type ClientOpt func(*Client) error

// StaticAccessToken makes an AccessTokenFunc that always returns the provided
// access token.
func StaticAccessToken(token string) AccessTokenFunc {
	return func(_ context.Context) (AccessToken, error) {
		return AccessToken(token), nil
	}
}

// WithHTTP sets a custom HTTP client for sending requests.
//
// The default value is `http.DefaultClient`.
func WithHTTP(http *http.Client) ClientOpt {
	return func(c *Client) error {
		c.http = http
		return nil
	}
}

// WithBaseURL sets a custom Recurse API URL.
//
// The default value is "https://www.recurse.com/api/v1".
func WithBaseURL(baseURL string) ClientOpt {
	return func(c *Client) error {
		u, err := url.Parse(baseURL)
		if err != nil {
			return err
		}

		c.baseURL = u
		return nil
	}
}

// ResponseError is the type of error returned when the response status
// indicates an error (400 or greater).
type ResponseError struct {
	Response *http.Response
}

func (r *ResponseError) Error() string {
	return fmt.Sprintf("error response from Recurse: %s", r.Response.Status)
}

// must panics if err is non-nil and returns val otherwise.
func must[T any](val T, err error) T {
	if err != nil {
		panic(err)
	}
	return val
}
