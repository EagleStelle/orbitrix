//go:build (darwin && !ios) || (linux && !android)

package platform

import (
	"os/exec"
	"strings"
)

// copyText hands s to the desktop's clipboard tool: pbcopy on macOS, wl-copy (Wayland) or xclip (X11) on Linux,
// whichever is installed.
func copyText(s string) bool {
	for _, tool := range [][]string{{"pbcopy"}, {"wl-copy"}, {"xclip", "-selection", "clipboard"}} {
		cmd := exec.Command(tool[0], tool[1:]...)
		cmd.Stdin = strings.NewReader(s)
		if cmd.Run() == nil {
			return true
		}
	}
	return false
}
