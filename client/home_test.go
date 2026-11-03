package client

import (
	"slices"
	"testing"
)

func TestWaysToPlay(t *testing.T) {
	const site = "https://orbitrix.example.com"
	for _, c := range []struct {
		name    string
		browser bool
		origin  string
		want    []string
	}{
		{"app without a server", false, "", []string{"This Device", "Same Wi-Fi"}},
		{"app with a server", false, site, []string{"This Device", "Same Wi-Fi", "Online"}},
		{"browser with a server", true, site, []string{"This Device", "Online"}},
		{"browser without a server", true, "", []string{"This Device"}},
	} {
		if got := waysToPlay(c.browser, c.origin); !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}
