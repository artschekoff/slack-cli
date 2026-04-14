package cli

import (
	"fmt"
	"time"
)

// ParseStartFrom parses an optional YYYY-MM-DD date string into a time.Time at
// midnight UTC. Returns zero time for empty input, error for invalid format.
func ParseStartFrom(dateStr string) (time.Time, error) {
	if dateStr == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid start_from date %q: expected YYYY-MM-DD format", dateStr)
	}
	return t, nil
}
