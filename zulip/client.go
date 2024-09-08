package zulip

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

var defaultBaseURL *url.URL = must(url.Parse("https://recurse.zulipchat.com/api/v1/"))

// Credentials are used to authenticate requests to the Zulip API.
type Credentials struct {
	Username string
	Password string
}

// CredentialsFunc returns the current Zulip API credentials.
type CredentialsFunc func(context.Context) (Credentials, error)

// A Client sends HTTP requests to the Zulip API.
type Client struct {
	http        *http.Client
	baseURL     *url.URL
	credentials CredentialsFunc
}

// NewClient creates a new Zulip API client.
func NewClient(credentials CredentialsFunc, opts ...ClientOpt) (*Client, error) {
	client := Client{
		http:        http.DefaultClient,
		baseURL:     defaultBaseURL,
		credentials: credentials,
	}

	for i, opt := range opts {
		if err := opt(&client); err != nil {
			return nil, fmt.Errorf("zulip client option %d: %w", i, err)
		}
	}

	return &client, nil
}

// PostToTopic sends a chat message to the given stream and topic.
func (c *Client) PostToTopic(ctx context.Context, stream, topic, message string) error {
	// TODO(@jdkaplan): Move this check to app logic / config
	if os.Getenv("APP_ENV") != "production" {
		log.Printf("In the Prod environment Pairing Bot would have posted the following message to %q > %q: %q", stream, topic, message)
		return nil
	}

	endpoint := c.baseURL.JoinPath("messages")

	form := make(url.Values)
	form.Add("type", "stream")
	form.Add("to", stream)
	form.Add("topic", topic)
	form.Add("content", message)

	return c.postForm(ctx, endpoint, form)
}

// SendUserMessage sends a (group) direct message to a set of users.
func (c *Client) SendUserMessage(ctx context.Context, userIDs []int64, message string) error {
	endpoint := c.baseURL.JoinPath("messages")

	form := make(url.Values)
	form.Add("type", "private")
	form.Add("to", recipients(userIDs))
	form.Add("content", message)

	return c.postForm(ctx, endpoint, form)
}

// recipients returns the string-array-of-strings required by the Zulip
// messaging API.
//
// Example:
//
//	recipients([]int64{1, 2, 3}) == "[1,2,3]"
func recipients(userIDs []int64) string {
	var users []string
	for _, id := range userIDs {
		users = append(users, strconv.FormatInt(id, 10))
	}
	return fmt.Sprintf("[%s]", strings.Join(users, ","))
}

// postForm sends the POST request with authorization and encoded form values.
// This returns a non-nil error if the response status code indicates an error
// (400 or higher) or if the request could not be sent.
func (c *Client) postForm(ctx context.Context, endpoint *url.URL, form url.Values) error {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint.String(),
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("content-type", "application/x-www-form-urlencoded")

	creds, err := c.credentials(ctx)
	if err != nil {
		return fmt.Errorf("fetch credentials: %w", err)
	}
	req.SetBasicAuth(creds.Username, creds.Password)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// This read will consume the body...
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	// ... so replace the content afterward.
	resp.Body = io.NopCloser(bytes.NewReader(body))

	log.Printf("zulip response: %d %s\n", resp.StatusCode, string(body))

	if resp.StatusCode >= 400 {
		return &ResponseError{resp}
	}
	return nil
}

// A ClientOpt is used to configure a Client.
type ClientOpt func(*Client) error

// StaticCredentials makes a CredentialsFunc that always returns the provided
// values.
func StaticCredentials(username string, password string) CredentialsFunc {
	return func(_ context.Context) (Credentials, error) {
		return Credentials{Username: username, Password: password}, nil
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

// WithBaseURL sets a custom Zulip API destination for messages.
//
// The default value is "https://recurse.zulipchat.com/api/v1".
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
	return fmt.Sprintf("error response from Zulip: %s", r.Response.Status)
}

// must panics if err is non-nil and returns val otherwise.
func must[T any](val T, err error) T {
	if err != nil {
		panic(err)
	}
	return val
}
