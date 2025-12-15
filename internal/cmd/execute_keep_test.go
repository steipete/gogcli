package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/api/keep/v1"
	"google.golang.org/api/option"
)

func TestExecute_KeepList_JSON(t *testing.T) {
	origNew := newKeepService
	t.Cleanup(func() { newKeepService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !(r.URL.Path == "/v1/notes" && r.Method == http.MethodGet) {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"notes": []map[string]any{
				{"name": "notes/n1", "title": "One", "updateTime": "2025-12-15T00:00:00Z"},
				{"name": "notes/n2", "title": "Two", "updateTime": "2025-12-15T00:00:01Z"},
			},
			"nextPageToken": "p2",
		})
	}))
	defer srv.Close()

	svc, err := keep.NewService(context.Background(),
		option.WithoutAuthentication(),
		option.WithHTTPClient(srv.Client()),
		option.WithEndpoint(srv.URL+"/"),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	newKeepService = func(context.Context, string) (*keep.Service, error) { return svc, nil }

	out := captureStdout(t, func() {
		_ = captureStderr(t, func() {
			if err := Execute([]string{"--output", "json", "--account", "a@b.com", "keep", "list", "--page-size", "10"}); err != nil {
				t.Fatalf("Execute: %v", err)
			}
		})
	})

	var parsed struct {
		Notes []struct {
			Name  string `json:"name"`
			Title string `json:"title"`
		} `json:"notes"`
		NextPageToken string `json:"nextPageToken"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json parse: %v\nout=%q", err, out)
	}
	if len(parsed.Notes) != 2 || parsed.Notes[0].Name != "notes/n1" || parsed.Notes[1].Name != "notes/n2" {
		t.Fatalf("unexpected notes: %#v", parsed.Notes)
	}
	if parsed.NextPageToken != "p2" {
		t.Fatalf("unexpected nextPageToken: %q", parsed.NextPageToken)
	}
}

func TestExecute_KeepGet_JSON(t *testing.T) {
	origNew := newKeepService
	t.Cleanup(func() { newKeepService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !(r.URL.Path == "/v1/notes/n1" && r.Method == http.MethodGet) {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"name":       "notes/n1",
			"title":      "Hello",
			"updateTime": "2025-12-15T00:00:00Z",
			"body": map[string]any{
				"text": map[string]any{"text": "World"},
			},
		})
	}))
	defer srv.Close()

	svc, err := keep.NewService(context.Background(),
		option.WithoutAuthentication(),
		option.WithHTTPClient(srv.Client()),
		option.WithEndpoint(srv.URL+"/"),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	newKeepService = func(context.Context, string) (*keep.Service, error) { return svc, nil }

	out := captureStdout(t, func() {
		_ = captureStderr(t, func() {
			if err := Execute([]string{"--output", "json", "--account", "a@b.com", "keep", "get", "notes/n1"}); err != nil {
				t.Fatalf("Execute: %v", err)
			}
		})
	})

	var parsed struct {
		Note struct {
			Name string `json:"name"`
		} `json:"note"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json parse: %v\nout=%q", err, out)
	}
	if parsed.Note.Name != "notes/n1" {
		t.Fatalf("unexpected note: %#v", parsed.Note)
	}
}

func TestExecute_KeepCreate_JSON(t *testing.T) {
	origNew := newKeepService
	t.Cleanup(func() { newKeepService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !(r.URL.Path == "/v1/notes" && r.Method == http.MethodPost) {
			http.NotFound(w, r)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(body["title"].(string)) != "Hello" {
			http.Error(w, "expected title Hello", http.StatusBadRequest)
			return
		}
		text := ""
		if b, ok := body["body"].(map[string]any); ok {
			if t, ok := b["text"].(map[string]any); ok {
				if v, ok := t["text"].(string); ok {
					text = v
				}
			}
		}
		if text != "World" {
			http.Error(w, "expected text World", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"name":  "notes/n1",
			"title": "Hello",
		})
	}))
	defer srv.Close()

	svc, err := keep.NewService(context.Background(),
		option.WithoutAuthentication(),
		option.WithHTTPClient(srv.Client()),
		option.WithEndpoint(srv.URL+"/"),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	newKeepService = func(context.Context, string) (*keep.Service, error) { return svc, nil }

	out := captureStdout(t, func() {
		_ = captureStderr(t, func() {
			if err := Execute([]string{"--output", "json", "--account", "a@b.com", "keep", "create", "--title", "Hello", "--text", "World"}); err != nil {
				t.Fatalf("Execute: %v", err)
			}
		})
	})

	var parsed struct {
		Note struct {
			Name  string `json:"name"`
			Title string `json:"title"`
		} `json:"note"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json parse: %v\nout=%q", err, out)
	}
	if parsed.Note.Name != "notes/n1" || parsed.Note.Title != "Hello" {
		t.Fatalf("unexpected note: %#v", parsed.Note)
	}
}

func TestExecute_KeepDelete_JSON(t *testing.T) {
	origNew := newKeepService
	t.Cleanup(func() { newKeepService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !(r.URL.Path == "/v1/notes/n1" && r.Method == http.MethodDelete) {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{})
	}))
	defer srv.Close()

	svc, err := keep.NewService(context.Background(),
		option.WithoutAuthentication(),
		option.WithHTTPClient(srv.Client()),
		option.WithEndpoint(srv.URL+"/"),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	newKeepService = func(context.Context, string) (*keep.Service, error) { return svc, nil }

	out := captureStdout(t, func() {
		_ = captureStderr(t, func() {
			if err := Execute([]string{"--output", "json", "--account", "a@b.com", "keep", "delete", "notes/n1"}); err != nil {
				t.Fatalf("Execute: %v", err)
			}
		})
	})

	var parsed struct {
		Deleted bool   `json:"deleted"`
		Name    string `json:"name"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json parse: %v\nout=%q", err, out)
	}
	if !parsed.Deleted || parsed.Name != "notes/n1" {
		t.Fatalf("unexpected response: %#v", parsed)
	}
}
