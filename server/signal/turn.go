package signal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TURN issues and revokes relay credentials. Cloudflare implements it; tests use a fake.
type TURN interface {
	Issue(ctx context.Context) (Credential, error)
	Revoke(ctx context.Context, username string) error
}

// Credential is one issued TURN credential and the ICE servers that carry it.
type Credential struct {
	Username   string
	ICEServers json.RawMessage
}

// credTTL covers a long session, not days.
const credTTL = 6 * time.Hour

// Cloudflare is Cloudflare Realtime TURN, from a TURN key's id and API token.
type Cloudflare struct {
	KeyID, Token string
	HTTP         *http.Client // nil means a client with a 10 s timeout
	api          string       // tests point it elsewhere
}

type iceServer struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
}

func (cf *Cloudflare) Issue(ctx context.Context) (Credential, error) {
	body := fmt.Sprintf(`{"ttl":%d}`, int(credTTL/time.Second))
	resp, err := cf.do(ctx, "credentials/generate-ice-servers", strings.NewReader(body))
	if err != nil {
		return Credential{}, err
	}
	defer resp.Body.Close()
	var out struct{ ICEServers []iceServer }
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&out); err != nil {
		return Credential{}, fmt.Errorf("turn: issue: %w", err)
	}
	var c Credential
	for i := range out.ICEServers {
		s := &out.ICEServers[i]
		if s.Username != "" {
			c.Username = s.Username
		}
		// Without trickle ICE, browsers stall on the alternate port 53, which they block (Cloudflare's advice).
		keep := s.URLs[:0]
		for _, u := range s.URLs {
			if !strings.Contains(u, ":53?") && !strings.HasSuffix(u, ":53") {
				keep = append(keep, u)
			}
		}
		s.URLs = keep
	}
	if c.Username == "" {
		return Credential{}, fmt.Errorf("turn: issue: no credential in the response")
	}
	c.ICEServers, err = json.Marshal(out.ICEServers)
	return c, err
}

func (cf *Cloudflare) Revoke(ctx context.Context, username string) error {
	resp, err := cf.do(ctx, "credentials/"+url.PathEscape(username)+"/revoke", nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// do POSTs to the key's endpoint and fails on any non-2xx status. It never puts the token or a credential in errors.
func (cf *Cloudflare) do(ctx context.Context, path string, body io.Reader) (*http.Response, error) {
	api := cf.api
	if api == "" {
		api = "https://rtc.live.cloudflare.com/v1"
	}
	if body == nil {
		body = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, api+"/turn/keys/"+url.PathEscape(cf.KeyID)+"/"+path, body)
	if err != nil {
		return nil, fmt.Errorf("turn: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cf.Token)
	req.Header.Set("Content-Type", "application/json")
	hc := cf.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := hc.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err // the URL names the credential being revoked
		}
		return nil, fmt.Errorf("turn: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		resp.Body.Close()
		return nil, fmt.Errorf("turn: HTTP %d", resp.StatusCode)
	}
	return resp, nil
}
