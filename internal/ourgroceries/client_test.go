package ourgroceries

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestClient points a Client at a test server instead of the live API.
func newTestClient(baseURL string) *Client {
	c := New("user@example.com", "secret")
	c.baseURL = baseURL
	return c
}

func TestAddItems(t *testing.T) {
	var signInHits, listHits int
	var gotCommand string
	var gotItems []map[string]any
	var gotTeamID string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/sign-in":
			signInHits++
			http.SetCookie(w, &http.Cookie{Name: "ourgroceries-auth", Value: "session-xyz"})
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/your-lists/" && r.Method == http.MethodGet:
			w.Write([]byte(`<html><script>var g_teamId = "team-42";</script></html>`))
		case r.URL.Path == "/your-lists/" && r.Method == http.MethodPost:
			listHits++
			body, _ := io.ReadAll(r.Body)
			var payload struct {
				Command string           `json:"command"`
				TeamID  string           `json:"teamId"`
				Items   []map[string]any `json:"items"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Errorf("bad json payload: %v", err)
			}
			gotCommand = payload.Command
			gotTeamID = payload.TeamID
			gotItems = payload.Items
			w.Write([]byte(`{}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	err := c.AddItems(context.Background(), "list-1", []Item{
		{Value: "Mushrooms", Note: "625g"},
		{Value: "", Note: "skip me"}, // empty value should be dropped
		{Value: "Salt"},              // no note
	})
	if err != nil {
		t.Fatalf("AddItems: %v", err)
	}

	if signInHits != 1 {
		t.Errorf("sign-in hits = %d, want 1", signInHits)
	}
	if listHits != 1 {
		t.Errorf("insertItems posts = %d, want 1", listHits)
	}
	if gotCommand != "insertItems" {
		t.Errorf("command = %q, want insertItems", gotCommand)
	}
	if gotTeamID != "team-42" {
		t.Errorf("teamId = %q, want team-42", gotTeamID)
	}
	if len(gotItems) != 2 {
		t.Fatalf("items len = %d, want 2 (empty value dropped)", len(gotItems))
	}
	if gotItems[0]["value"] != "Mushrooms" || gotItems[0]["note"] != "625g" || gotItems[0]["listId"] != "list-1" {
		t.Errorf("item[0] = %+v, want value=Mushrooms note=625g listId=list-1", gotItems[0])
	}
	if gotItems[1]["value"] != "Salt" {
		t.Errorf("item[1] value = %v, want Salt", gotItems[1]["value"])
	}
	if _, hasNote := gotItems[1]["note"]; hasNote {
		t.Errorf("item[1] should have no note, got %+v", gotItems[1])
	}
}

func TestAddItemsEmptyBatchIsNoop(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("no request expected for empty batch, got %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	if err := c.AddItems(context.Background(), "list-1", []Item{{Value: "  "}}); err != nil {
		t.Fatalf("AddItems empty batch: %v", err)
	}
}

func TestGetList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/sign-in":
			http.SetCookie(w, &http.Cookie{Name: "ourgroceries-auth", Value: "session-xyz"})
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/your-lists/" && r.Method == http.MethodGet:
			w.Write([]byte(`var g_teamId = "team-42";`))
		case r.URL.Path == "/your-lists/" && r.Method == http.MethodPost:
			var payload struct {
				Command string `json:"command"`
				ListID  string `json:"listId"`
			}
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &payload)
			if payload.Command != "getList" || payload.ListID != "list-1" {
				t.Errorf("unexpected payload: command=%q listId=%q", payload.Command, payload.ListID)
			}
			w.Write([]byte(`{"list":{"items":[
				{"value":"Mushrooms","note":"625g"},
				{"value":"Milk","note":"2l","crossedOff":true}
			]}}`))
		}
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	items, err := c.GetList(context.Background(), "list-1")
	if err != nil {
		t.Fatalf("GetList: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items len = %d, want 2", len(items))
	}
	if items[0] != (ListItem{Value: "Mushrooms", Note: "625g", CrossedOff: false}) {
		t.Errorf("item[0] = %+v", items[0])
	}
	if items[1] != (ListItem{Value: "Milk", Note: "2l", CrossedOff: true}) {
		t.Errorf("item[1] = %+v", items[1])
	}
}

func TestAddItemsReauthOnFailure(t *testing.T) {
	var signInHits, postAttempts int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/sign-in":
			signInHits++
			http.SetCookie(w, &http.Cookie{Name: "ourgroceries-auth", Value: "session-xyz"})
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/your-lists/" && r.Method == http.MethodGet:
			w.Write([]byte(`var g_teamId = "team-42";`))
		case r.URL.Path == "/your-lists/" && r.Method == http.MethodPost:
			postAttempts++
			if postAttempts == 1 {
				w.WriteHeader(http.StatusUnauthorized) // simulate expired session
				return
			}
			w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	if err := c.AddItems(context.Background(), "list-1", []Item{{Value: "Eggs"}}); err != nil {
		t.Fatalf("AddItems with reauth: %v", err)
	}
	if signInHits != 2 {
		t.Errorf("sign-in hits = %d, want 2 (initial + reauth)", signInHits)
	}
	if postAttempts != 2 {
		t.Errorf("post attempts = %d, want 2 (fail + retry)", postAttempts)
	}
}
