//go:build (!windows && !js && !darwin && !linux) || ios || android

package platform

// copyText has no clipboard to use here.
func copyText(string) bool { return false }
