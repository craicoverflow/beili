// Package ourgroceries is a minimal client for the (unofficial) OurGroceries
// web API. It exists so Béilí can push shopping items straight into a shared
// OurGroceries list — including a per-item note carrying quantity/form metadata
// that a downstream Tesco basket builder uses to match the right product. This
// replaces the older name-only Home Assistant shopping-list webhook hop, which
// could not carry that metadata.
package ourgroceries

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"sync"
)

const defaultBaseURL = "https://www.ourgroceries.com"

// teamIDRe extracts the team id embedded in the your-lists page HTML, e.g.
// `g_teamId = "abc123";`.
var teamIDRe = regexp.MustCompile(`g_teamId = "(.*?)";`)

// Item is a single shopping-list entry. Note carries metadata (quantity, form)
// that the downstream Tesco basket builder uses to match a product.
type Item struct {
	Value string // item name, e.g. "Mushrooms"
	Note  string // metadata, e.g. "625g"
}

// Client talks to the OurGroceries web API. It is safe for concurrent use.
type Client struct {
	baseURL  string
	email    string
	password string
	http     *http.Client

	mu     sync.Mutex
	teamID string
	authed bool
}

// New returns a Client authenticating with the given OurGroceries credentials.
// Login is lazy — it happens on the first command.
func New(email, password string) *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		baseURL:  defaultBaseURL,
		email:    email,
		password: password,
		http:     &http.Client{Jar: jar},
	}
}

// AddItems adds items to the given list in a single insertItems command. Items
// with an empty Value are skipped; an empty resulting batch is a no-op. If the
// command fails (e.g. an expired session) it re-authenticates once and retries.
func (c *Client) AddItems(ctx context.Context, listID string, items []Item) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	payloadItems := make([]map[string]any, 0, len(items))
	for _, it := range items {
		if strings.TrimSpace(it.Value) == "" {
			continue
		}
		entry := map[string]any{
			"value":  it.Value,
			"listId": listID,
		}
		if it.Note != "" {
			entry["note"] = it.Note
		}
		payloadItems = append(payloadItems, entry)
	}
	if len(payloadItems) == 0 {
		return nil
	}

	send := func() error {
		return c.post(ctx, map[string]any{
			"command": "insertItems",
			"items":   payloadItems,
		})
	}

	if err := c.ensureAuth(ctx); err != nil {
		return err
	}
	if err := send(); err != nil {
		// The session may have expired since we last authenticated; drop it,
		// re-authenticate once and retry before giving up.
		c.authed = false
		if reauthErr := c.ensureAuth(ctx); reauthErr != nil {
			return err
		}
		return send()
	}
	return nil
}

func (c *Client) ensureAuth(ctx context.Context) error {
	if c.authed {
		return nil
	}
	return c.login(ctx)
}

// login authenticates, capturing the session cookie (via the cookie jar) and
// the team id (scraped from the your-lists page).
func (c *Client) login(ctx context.Context) error {
	form := url.Values{
		"emailAddress": {c.email},
		"password":     {c.password},
		"action":       {"sign-in"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/sign-in", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("ourgroceries sign-in: %w", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("ourgroceries sign-in: status %d", resp.StatusCode)
	}

	teamID, err := c.fetchTeamID(ctx)
	if err != nil {
		return err
	}
	c.teamID = teamID
	c.authed = true
	return nil
}

func (c *Client) fetchTeamID(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/your-lists/", nil)
	if err != nil {
		return "", err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("ourgroceries your-lists: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	m := teamIDRe.FindSubmatch(body)
	if m == nil {
		return "", fmt.Errorf("ourgroceries: team id not found in your-lists page (login likely failed)")
	}
	return string(m[1]), nil
}

// post sends a JSON command to the your-lists endpoint, merging in the team id.
func (c *Client) post(ctx context.Context, payload map[string]any) error {
	payload["teamId"] = c.teamID
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/your-lists/", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("ourgroceries command %v: status %d", payload["command"], resp.StatusCode)
	}
	return nil
}
