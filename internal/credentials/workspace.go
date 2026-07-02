package credentials

import (
	"fmt"
	"regexp"
)

var workspaceNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// ValidateWorkspaceName checks that name is a valid workspace identifier:
// lowercase alphanumeric + hyphens, 1–64 chars, must start with a letter or digit.
func ValidateWorkspaceName(name string) error {
	if !workspaceNameRe.MatchString(name) {
		return fmt.Errorf("invalid workspace name %q: must match %s", name, workspaceNameRe.String())
	}
	return nil
}
