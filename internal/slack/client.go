// Package slack provides a typed HTTP client for the Slack Web API.
package slack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	slackAPIBase      = "https://slack.com/api/"
	defaultTimeout    = 30 * time.Second
	maxTextLength     = 120
	maxRetryAfterSec  = 60
	maxResponseBytes  = 10 << 20 // 10 MB cap to prevent unbounded reads
)

// ErrRateLimited is returned when Slack responds with HTTP 429 and a retry also fails,
// or when Slack returns ok=false with error "ratelimited".
var ErrRateLimited = errors.New("slack rate limit exceeded")

// ErrUnauthorized is returned when auth.test returns ok=false due to invalid credentials.
var ErrUnauthorized = errors.New("slack credentials invalid or expired")

// ErrNoChannelAccess is returned when the token lacks permissions for the requested channel.
var ErrNoChannelAccess = errors.New("token lacks permissions for this channel")

// ErrUnexpected is returned when Slack returns ok=false with an unrecognised error code.
// The raw code is intentionally withheld to avoid leaking internal API details.
var ErrUnexpected = errors.New("slack API returned an unexpected error")

// Client wraps an http.Client with Slack authentication credentials.
type Client struct {
	token   string
	cookie  string
	http    *http.Client
	baseURL string // defaults to slackAPIBase; overridable via WithBaseURL
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithBaseURL overrides the Slack API base URL. Intended for tests.
func WithBaseURL(url string) ClientOption {
	return func(c *Client) { c.baseURL = url }
}

// NewClient creates a Client using the given xoxc- token and xoxd- cookie.
func NewClient(token, cookie string, opts ...ClientOption) *Client {
	c := &Client{
		token:   token,
		cookie:  cookie,
		http:    &http.Client{Timeout: defaultTimeout},
		baseURL: slackAPIBase,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// get performs an authenticated GET request to the Slack API.
// Handles HTTP 429 rate-limiting with a single retry honoring the Retry-After header.
func (c *Client) get(ctx context.Context, endpoint string, params url.Values) ([]byte, error) {
	reqURL := c.baseURL + endpoint
	if len(params) > 0 {
		reqURL += "?" + params.Encode()
	}

	body, retryAfterSec, err := c.doRequest(ctx, reqURL)
	if err != nil {
		return nil, err
	}

	if retryAfterSec > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(retryAfterSec) * time.Second):
		}

		body, retryAfterSec, err = c.doRequest(ctx, reqURL)
		if err != nil {
			return nil, err
		}
		if retryAfterSec > 0 {
			return nil, ErrRateLimited
		}
	}

	return body, nil
}

// doRequest executes a single GET request.
// Returns (body, retryAfterSeconds, error); retryAfterSeconds > 0 means HTTP 429 was received.
func (c *Client) doRequest(ctx context.Context, reqURL string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("building request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.AddCookie(&http.Cookie{Name: "d", Value: c.cookie})

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, retryAfter, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("slack API returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, 0, fmt.Errorf("reading response body: %w", err)
	}

	return body, 0, nil
}

// parseRetryAfter parses the Retry-After header value into seconds (defaults to 1).
// The result is capped at maxRetryAfterSec to prevent a misbehaving server from
// blocking the client indefinitely.
func parseRetryAfter(header string) int {
	if header == "" {
		return 1
	}
	v, err := strconv.Atoi(strings.TrimSpace(header))
	if err != nil || v < 1 {
		return 1
	}
	if v > maxRetryAfterSec {
		return maxRetryAfterSec
	}
	return v
}

// slackBaseResponse is embedded in all Slack API response structs.
type slackBaseResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// checkResponse returns an error if the Slack API returned ok=false.
// Known error codes are mapped to sentinel errors; unrecognised codes return
// ErrUnexpected to avoid leaking raw Slack API details to callers.
func checkResponse(base slackBaseResponse) error {
	if base.OK {
		return nil
	}
	switch base.Error {
	case "not_in_channel", "channel_not_found", "missing_scope":
		return ErrNoChannelAccess
	case "invalid_auth", "not_authed", "account_inactive", "token_revoked":
		return ErrUnauthorized
	case "ratelimited":
		return ErrRateLimited
	default:
		return ErrUnexpected
	}
}

// unmarshal decodes JSON into v, wrapping any error.
func unmarshal(data []byte, v any) error {
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("decoding Slack response: %w", err)
	}
	return nil
}

// TruncateText truncates s to maxTextLength runes, appending "…" if truncated.
func TruncateText(s string) string {
	runes := []rune(s)
	if len(runes) <= maxTextLength {
		return s
	}
	return string(runes[:maxTextLength]) + "…"
}

// FormatTS converts a Slack timestamp string (e.g. "1700000000.123456") to a human-readable UTC time.
func FormatTS(ts string) string {
	dotIdx := strings.Index(ts, ".")
	if dotIdx < 0 {
		return ts
	}
	sec, err := strconv.ParseInt(ts[:dotIdx], 10, 64)
	if err != nil {
		return ts
	}
	return time.Unix(sec, 0).UTC().Format("2006-01-02 15:04 UTC")
}
