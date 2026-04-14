package slack

import (
	"context"
	"fmt"
)

// AuthTestResult holds the parsed response from auth.test.
type AuthTestResult struct {
	UserID string
	User   string
	TeamID string
	Team   string
	URL    string
}

type authTestResponse struct {
	slackBaseResponse
	UserID string `json:"user_id"`
	User   string `json:"user"`
	TeamID string `json:"team_id"`
	Team   string `json:"team"`
	URL    string `json:"url"`
}

// ValidateToken calls auth.test and returns the authenticated user info.
// Returns ErrUnauthorized if the credentials are invalid or expired.
func (c *Client) ValidateToken(ctx context.Context) (AuthTestResult, error) {
	body, err := c.get(ctx, "auth.test", nil)
	if err != nil {
		return AuthTestResult{}, fmt.Errorf("auth.test request: %w", err)
	}

	var resp authTestResponse
	if err := unmarshal(body, &resp); err != nil {
		return AuthTestResult{}, err
	}

	if err := checkResponse(resp.slackBaseResponse); err != nil {
		return AuthTestResult{}, err
	}

	return AuthTestResult{
		UserID: resp.UserID,
		User:   resp.User,
		TeamID: resp.TeamID,
		Team:   resp.Team,
		URL:    resp.URL,
	}, nil
}
