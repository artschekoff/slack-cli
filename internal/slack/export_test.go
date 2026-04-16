// Package slack — this file is compiled only during tests.
package slack

import (
	"context"
	"net/url"
)

// CheckResponse exports checkResponse for white-box testing.
var CheckResponse = checkResponse

// ParseRetryAfter exports parseRetryAfter for white-box testing.
var ParseRetryAfter = parseRetryAfter

// SlackBaseResponse exports slackBaseResponse for white-box testing.
type SlackBaseResponse = slackBaseResponse

// MaxPages exports maxPages for white-box testing.
const MaxPages = maxPages

// ExportedPaginatedFetch exposes paginatedFetch for white-box testing.
func (c *Client) ExportedPaginatedFetch(ctx context.Context, endpoint string, buildParams func(cursor string) url.Values) ([]Message, bool, error) {
	return c.paginatedFetch(ctx, endpoint, buildParams)
}

// TsOnOrAfter exports tsOnOrAfter for white-box testing.
var TsOnOrAfter = tsOnOrAfter

// TimeToSlackTS exports timeToSlackTS for white-box testing.
var TimeToSlackTS = timeToSlackTS
