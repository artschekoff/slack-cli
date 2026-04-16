package cli

import (
	"errors"
	"fmt"
)

// Sentinel errors returned by CLI commands.
// MCP handlers use errors.Is against these to map to user-safe messages.
var (
	ErrUnauthorized        = errors.New("credentials expired")
	ErrNoChannelAccess     = errors.New("no channel access")
	ErrSlackSearch         = errors.New("search failed")
	ErrSlackLoadThread     = errors.New("failed to load thread")
	ErrSlackGetUser        = errors.New("failed to get user info")
	ErrCredentialsNotFound = errors.New("credentials not found")
)

// unauthorizedError carries the workspace name so the error message is actionable.
type unauthorizedError struct{ workspace string }

func (e *unauthorizedError) Error() string {
	return fmt.Sprintf("credentials for workspace '%s' are expired. Use auth_start to re-authenticate.", e.workspace)
}

func (e *unauthorizedError) Is(target error) bool { return target == ErrUnauthorized }

// noChannelAccessError matches the message expected by existing MCP tests.
type noChannelAccessError struct{}

func (e *noChannelAccessError) Error() string { return "token lacks permissions for this channel" }

func (e *noChannelAccessError) Is(target error) bool { return target == ErrNoChannelAccess }
