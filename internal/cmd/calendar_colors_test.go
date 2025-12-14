package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

func TestCalendarColorsCmd_JSON(t *testing.T) {
	origNew := newCalendarService
	t.Cleanup(func() { newCalendarService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/colors") && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"event": map[string]any{
					"1": map[string]string{
						"background": "#a4bdfc",
						"foreground": "#1d1d1d",
					},
					"2": map[string]string{
						"background": "#7ae7bf",
						"foreground": "#1d1d1d",
					},
				},
				"calendar": map[string]any{
					"1": map[string]string{
						"background": "#ac725e",
						"foreground": "#1d1d1d",
					},
					"2": map[string]string{
						"background": "#d06b64",
						"foreground": "#1d1d1d",
					},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	svc, err := calendar.NewService(context.Background(),
		option.WithoutAuthentication(),
		option.WithHTTPClient(srv.Client()),
		option.WithEndpoint(srv.URL+"/"),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	newCalendarService = func(context.Context, string) (*calendar.Service, error) { return svc, nil }

	out := captureStdout(t, func() {
		_ = captureStderr(t, func() {
			if err := Execute([]string{"--output", "json", "--account", "a@b.com", "calendar", "colors"}); err != nil {
				t.Fatalf("Execute: %v", err)
			}
		})
	})

	var parsed struct {
		Event    map[string]struct {
			Background string `json:"background"`
			Foreground string `json:"foreground"`
		} `json:"event"`
		Calendar map[string]struct {
			Background string `json:"background"`
			Foreground string `json:"foreground"`
		} `json:"calendar"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json parse: %v\nout=%q", err, out)
	}

	// Verify event colors
	if len(parsed.Event) != 2 {
		t.Fatalf("expected 2 event colors, got %d", len(parsed.Event))
	}
	if parsed.Event["1"].Background != "#a4bdfc" {
		t.Errorf("unexpected event color 1 background: %q", parsed.Event["1"].Background)
	}
	if parsed.Event["1"].Foreground != "#1d1d1d" {
		t.Errorf("unexpected event color 1 foreground: %q", parsed.Event["1"].Foreground)
	}

	// Verify calendar colors
	if len(parsed.Calendar) != 2 {
		t.Fatalf("expected 2 calendar colors, got %d", len(parsed.Calendar))
	}
	if parsed.Calendar["1"].Background != "#ac725e" {
		t.Errorf("unexpected calendar color 1 background: %q", parsed.Calendar["1"].Background)
	}
	if parsed.Calendar["1"].Foreground != "#1d1d1d" {
		t.Errorf("unexpected calendar color 1 foreground: %q", parsed.Calendar["1"].Foreground)
	}
}

func TestCalendarColorsCmd_TableOutput(t *testing.T) {
	origNew := newCalendarService
	t.Cleanup(func() { newCalendarService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/colors") && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"event": map[string]any{
					"1": map[string]string{
						"background": "#a4bdfc",
						"foreground": "#1d1d1d",
					},
					"2": map[string]string{
						"background": "#7ae7bf",
						"foreground": "#1d1d1d",
					},
				},
				"calendar": map[string]any{
					"1": map[string]string{
						"background": "#ac725e",
						"foreground": "#1d1d1d",
					},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	svc, err := calendar.NewService(context.Background(),
		option.WithoutAuthentication(),
		option.WithHTTPClient(srv.Client()),
		option.WithEndpoint(srv.URL+"/"),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	newCalendarService = func(context.Context, string) (*calendar.Service, error) { return svc, nil }

	out := captureStdout(t, func() {
		_ = captureStderr(t, func() {
			if err := Execute([]string{"--account", "a@b.com", "calendar", "colors"}); err != nil {
				t.Fatalf("Execute: %v", err)
			}
		})
	})

	// Verify table headers and content
	if !strings.Contains(out, "EVENT COLORS:") {
		t.Errorf("output missing event colors header: %q", out)
	}
	if !strings.Contains(out, "CALENDAR COLORS:") {
		t.Errorf("output missing calendar colors header: %q", out)
	}
	if !strings.Contains(out, "ID") {
		t.Errorf("output missing ID column header: %q", out)
	}
	if !strings.Contains(out, "BACKGROUND") {
		t.Errorf("output missing BACKGROUND column header: %q", out)
	}
	if !strings.Contains(out, "FOREGROUND") {
		t.Errorf("output missing FOREGROUND column header: %q", out)
	}

	// Verify color values appear in output
	if !strings.Contains(out, "#a4bdfc") {
		t.Errorf("output missing event color background: %q", out)
	}
	if !strings.Contains(out, "#ac725e") {
		t.Errorf("output missing calendar color background: %q", out)
	}
	if !strings.Contains(out, "#1d1d1d") {
		t.Errorf("output missing foreground color: %q", out)
	}
}

func TestCalendarColorsCmd_EmptyColors(t *testing.T) {
	origNew := newCalendarService
	t.Cleanup(func() { newCalendarService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/colors") && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"event":    map[string]any{},
				"calendar": map[string]any{},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	svc, err := calendar.NewService(context.Background(),
		option.WithoutAuthentication(),
		option.WithHTTPClient(srv.Client()),
		option.WithEndpoint(srv.URL+"/"),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	newCalendarService = func(context.Context, string) (*calendar.Service, error) { return svc, nil }

	// Test JSON output with empty colors
	outJSON := captureStdout(t, func() {
		_ = captureStderr(t, func() {
			if err := Execute([]string{"--output", "json", "--account", "a@b.com", "calendar", "colors"}); err != nil {
				t.Fatalf("Execute: %v", err)
			}
		})
	})

	var parsed struct {
		Event    map[string]any `json:"event"`
		Calendar map[string]any `json:"calendar"`
	}
	if err := json.Unmarshal([]byte(outJSON), &parsed); err != nil {
		t.Fatalf("json parse: %v\nout=%q", err, outJSON)
	}
	if len(parsed.Event) != 0 {
		t.Errorf("expected empty event colors, got %d", len(parsed.Event))
	}
	if len(parsed.Calendar) != 0 {
		t.Errorf("expected empty calendar colors, got %d", len(parsed.Calendar))
	}

	// Test table output with empty colors
	stderr := captureStderr(t, func() {
		_ = captureStdout(t, func() {
			if err := Execute([]string{"--account", "a@b.com", "calendar", "colors"}); err != nil {
				t.Fatalf("Execute: %v", err)
			}
		})
	})

	if !strings.Contains(stderr, "No colors available") {
		t.Errorf("expected 'No colors available' message, got: %q", stderr)
	}
}
