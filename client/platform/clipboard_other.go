//go:build !windows && !js

package platform

// copyText has no clipboard to use here.
func copyText(string) bool { return false }
