package signal

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCloudflare(t *testing.T) {
	var revoked string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer tok" {
			http.Error(w, "no", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/turn/keys/key1/credentials/generate-ice-servers":
			if b, _ := io.ReadAll(r.Body); string(b) != `{"ttl":21600}` {
				t.Errorf("body %s", b)
			}
			w.WriteHeader(http.StatusCreated)
			io.WriteString(w, `{"iceServers":[{"urls":["stun:stun.cloudflare.com:3478","stun:stun.cloudflare.com:53"]},
				{"urls":["turn:turn.cloudflare.com:3478?transport=udp","turn:turn.cloudflare.com:53?transport=udp",
				"turns:turn.cloudflare.com:443?transport=tcp"],"username":"abc123","credential":"secret"}]}`)
		case "/turn/keys/key1/credentials/abc123/revoke":
			revoked = "abc123"
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()

	ctx := context.Background()
	cf := &Cloudflare{KeyID: "key1", Token: "tok", api: api.URL}
	c, err := cf.Issue(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ice := string(c.ICEServers)
	if c.Username != "abc123" || strings.Contains(ice, ":53") || !strings.Contains(ice, "turns:turn.cloudflare.com:443") {
		t.Fatalf("credential %q, ICE %s", c.Username, ice)
	}
	if err := cf.Revoke(ctx, c.Username); err != nil || revoked != "abc123" {
		t.Fatalf("revoke: %v", err)
	}
	bad := &Cloudflare{KeyID: "key1", Token: "wrong", api: api.URL}
	if _, err := bad.Issue(ctx); err == nil || strings.Contains(err.Error(), "wrong") {
		t.Fatalf("bad token: %v", err)
	}
}
