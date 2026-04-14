package browser

import (
	"os/exec"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpen_UsesCorrectCommandPerOS(t *testing.T) {
	tests := []struct {
		goos    string
		wantCmd string
		wantArg string
	}{
		{"darwin", "open", "https://example.com"},
		{"linux", "xdg-open", "https://example.com"},
		{"windows", "cmd", "https://example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			origLookup := osLookup
			t.Cleanup(func() { osLookup = origLookup })
			osLookup = func() string { return tt.goos }

			var gotName string
			var gotArgs []string
			origExecCommand := execCommand
			t.Cleanup(func() { execCommand = origExecCommand })
			execCommand = func(name string, args ...string) *exec.Cmd {
				gotName = name
				gotArgs = args
				return exec.Command("echo") // harmless no-op
			}

			err := Open(tt.wantArg)
			require.NoError(t, err)
			assert.Equal(t, tt.wantCmd, gotName)
			if tt.goos == "windows" {
				assert.Equal(t, []string{"/c", "start", tt.wantArg}, gotArgs)
			} else {
				assert.Equal(t, []string{tt.wantArg}, gotArgs)
			}
		})
	}
}

func TestOpen_UnsupportedOS_ReturnsError(t *testing.T) {
	origLookup := osLookup
	t.Cleanup(func() { osLookup = origLookup })
	osLookup = func() string { return "plan9" }

	err := Open("https://example.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported platform")
	assert.Contains(t, err.Error(), "plan9")
}

func TestOpen_DefaultOSLookup_ReturnsRuntimeGOOS(t *testing.T) {
	assert.Equal(t, runtime.GOOS, defaultOSLookup())
}
