// Package ourgroceries is a minimal client for the (unofficial) OurGroceries
// web API. It exists so Béilí can push shopping items straight into a shared
// OurGroceries list, replacing the older Home Assistant shopping-list webhook
// hop. Items are pushed as "Name (amount)" values; the optional per-item note
// field is supported but currently unused by the push path.
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
		_, err := c.do(ctx, map[string]any{
			"command": "insertItems",
			"items":   payloadItems,
		})
		return err
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

// ListItem is a single entry read back from a list. CrossedOff reflects whether
// it has been checked off (defaults to false when absent from the response).
type ListItem struct {
	Value      string
	Note       string
	CrossedOff bool
}

// GetList fetches the current items in a list. Like AddItems, it re-authenticates
// once and retries if the session has expired.
func (c *Client) GetList(ctx context.Context, listID string) ([]ListItem, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.ensureAuth(ctx); err != nil {
		return nil, err
	}
	items, err := c.getList(ctx, listID)
	if err != nil {
		c.authed = false
		if reauthErr := c.ensureAuth(ctx); reauthErr != nil {
			return nil, err
		}
		return c.getList(ctx, listID)
	}
	return items, nil
}

func (c *Client) getList(ctx context.Context, listID string) ([]ListItem, error) {
	body, err := c.do(ctx, map[string]any{
		"command": "getList",
		"listId":  listID,
	})
	if err != nil {
		return nil, err
	}

	var resp struct {
		List struct {
			Items []struct {
				Value      string `json:"value"`
				Note       string `json:"note"`
				CrossedOff bool   `json:"crossedOff"`
			} `json:"items"`
		} `json:"list"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("ourgroceries getList: decode response: %w", err)
	}

	items := make([]ListItem, 0, len(resp.List.Items))
	for _, it := range resp.List.Items {
		items = append(items, ListItem{Value: it.Value, Note: it.Note, CrossedOff: it.CrossedOff})
	}
	return items, nil
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

// do sends a JSON command to the your-lists endpoint, merging in the team id,
// and returns the raw response body.
func (c *Client) do(ctx context.Context, payload map[string]any) ([]byte, error) {
	payload["teamId"] = c.teamID
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/your-lists/", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("ourgroceries command %v: status %d", payload["command"], resp.StatusCode)
	}
	return respBody, nil
}
