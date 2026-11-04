package client

import (
	"slices"
	"testing"
)

func TestWaysToPlay(t *testing.T) {
	const site = "https://orbitrix.example.com"
	for _, c := range []struct {
		name   string
		origin string
		want   []string
	}{
		{"without a server (an app built without one, a LAN host's page)", "", []string{"This Device", "Same Wi-Fi"}},
		{"with a server (an app built with one, the website)", site, []string{"This Device", "Same Wi-Fi", "Online"}},
	} {
		if got := waysToPlay(c.origin); !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}
