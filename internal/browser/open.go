// Package browser provides a cross-platform function to open a URL in the
// user's default browser.
package browser

import (
	"fmt"
	"os/exec"
	"runtime"
)

// osLookup returns the current OS. Swappable for tests.
var osLookup = defaultOSLookup

func defaultOSLookup() string { return runtime.GOOS }

// execCommand wraps exec.Command so tests can intercept the invocation.
var execCommand = exec.Command

// Open opens url in the default system browser.
// Supported platforms: darwin (open), linux (xdg-open), windows (cmd /c start).
func Open(url string) error {
	switch osLookup() {
	case "darwin":
		return execCommand("open", url).Start()
	case "linux":
		return execCommand("xdg-open", url).Start()
	case "windows":
		return execCommand("cmd", "/c", "start", url).Start()
	default:
		return fmt.Errorf("unsupported platform %q: cannot open browser", osLookup())
	}
}
