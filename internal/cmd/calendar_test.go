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

func TestSplitCSV(t *testing.T) {
	if got := splitCSV(""); got != nil {
		t.Fatalf("unexpected: %#v", got)
	}
	got := splitCSV(" a@b.com, c@d.com ,,")
	if len(got) != 2 || got[0] != "a@b.com" || got[1] != "c@d.com" {
		t.Fatalf("unexpected: %#v", got)
	}
}

func TestBuildEventDateTime(t *testing.T) {
	allDay := buildEventDateTime("2025-01-01", true)
	if allDay.Date != "2025-01-01" || allDay.DateTime != "" {
		t.Fatalf("unexpected: %#v", allDay)
	}
	timed := buildEventDateTime("2025-01-01T10:00:00Z", false)
	if timed.DateTime != "2025-01-01T10:00:00Z" || timed.Date != "" {
		t.Fatalf("unexpected: %#v", timed)
	}
}

func TestIsAllDayEvent(t *testing.T) {
	if isAllDayEvent(nil) {
		t.Fatalf("expected false")
	}
	if !isAllDayEvent(&calendar.Event{Start: &calendar.EventDateTime{Date: "2025-01-01"}}) {
		t.Fatalf("expected true")
	}
}

func TestCalendarDelete_Success(t *testing.T) {
	origNew := newCalendarService
	t.Cleanup(func() { newCalendarService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/calendars/cal1/events/evt1") && r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
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
			if err := Execute([]string{"--output", "json", "--account", "a@b.com", "calendar", "delete", "cal1", "evt1"}); err != nil {
				t.Fatalf("Execute: %v", err)
			}
		})
	})

	var parsed struct {
		Deleted    bool   `json:"deleted"`
		CalendarID string `json:"calendarId"`
		EventID    string `json:"eventId"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json parse: %v\nout=%q", err, out)
	}
	if !parsed.Deleted || parsed.CalendarID != "cal1" || parsed.EventID != "evt1" {
		t.Fatalf("unexpected result: %#v", parsed)
	}
}

func TestCalendarDelete_NotFound(t *testing.T) {
	origNew := newCalendarService
	t.Cleanup(func() { newCalendarService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/calendars/cal1/events/nonexistent") && r.Method == http.MethodDelete {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"code":    404,
					"message": "Not Found",
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

	err = Execute([]string{"--output", "json", "--account", "a@b.com", "calendar", "delete", "cal1", "nonexistent"})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "404") && !strings.Contains(err.Error(), "Not Found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCalendarDelete_APIError(t *testing.T) {
	origNew := newCalendarService
	t.Cleanup(func() { newCalendarService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/calendars/cal1/events/evt1") && r.Method == http.MethodDelete {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"code":    500,
					"message": "Internal Server Error",
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

	err = Execute([]string{"--output", "json", "--account", "a@b.com", "calendar", "delete", "cal1", "evt1"})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "500") && !strings.Contains(err.Error(), "Internal Server Error") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCalendarEvents_Success(t *testing.T) {
	origNew := newCalendarService
	t.Cleanup(func() { newCalendarService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/calendars/cal1/events") && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{
					{
						"id":      "evt1",
						"summary": "Meeting 1",
						"start":   map[string]any{"dateTime": "2025-12-13T10:00:00Z"},
						"end":     map[string]any{"dateTime": "2025-12-13T11:00:00Z"},
					},
					{
						"id":      "evt2",
						"summary": "Meeting 2",
						"start":   map[string]any{"dateTime": "2025-12-14T14:00:00Z"},
						"end":     map[string]any{"dateTime": "2025-12-14T15:00:00Z"},
					},
				},
				"nextPageToken": "",
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
			if err := Execute([]string{"--output", "json", "--account", "a@b.com", "calendar", "events", "cal1"}); err != nil {
				t.Fatalf("Execute: %v", err)
			}
		})
	})

	var parsed struct {
		Events []struct {
			ID      string `json:"id"`
			Summary string `json:"summary"`
		} `json:"events"`
		NextPageToken string `json:"nextPageToken"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json parse: %v\nout=%q", err, out)
	}
	if len(parsed.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(parsed.Events))
	}
	if parsed.Events[0].ID != "evt1" || parsed.Events[0].Summary != "Meeting 1" {
		t.Fatalf("unexpected first event: %#v", parsed.Events[0])
	}
}

func TestCalendarEvents_EmptyList(t *testing.T) {
	origNew := newCalendarService
	t.Cleanup(func() { newCalendarService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/calendars/cal1/events") && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items":         []map[string]any{},
				"nextPageToken": "",
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
			if err := Execute([]string{"--output", "json", "--account", "a@b.com", "calendar", "events", "cal1"}); err != nil {
				t.Fatalf("Execute: %v", err)
			}
		})
	})

	var parsed struct {
		Events        []any  `json:"events"`
		NextPageToken string `json:"nextPageToken"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json parse: %v\nout=%q", err, out)
	}
	if len(parsed.Events) != 0 {
		t.Fatalf("expected empty events list, got %d events", len(parsed.Events))
	}
}

func TestCalendarACL_ListRoles(t *testing.T) {
	origNew := newCalendarService
	t.Cleanup(func() { newCalendarService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/calendars/primary/acl") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{
					"id":   "user:owner@example.com",
					"role": "owner",
					"scope": map[string]any{
						"type":  "user",
						"value": "owner@example.com",
					},
				},
				{
					"id":   "user:reader@example.com",
					"role": "reader",
					"scope": map[string]any{
						"type":  "user",
						"value": "reader@example.com",
					},
				},
			},
		})
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
			if err := Execute([]string{"--output", "text", "--account", "a@b.com", "calendar", "acl", "primary"}); err != nil {
				t.Fatalf("Execute: %v", err)
			}
		})
	})

	if !strings.Contains(out, "SCOPE_TYPE") || !strings.Contains(out, "ROLE") {
		t.Fatalf("unexpected output headers: %q", out)
	}
	if !strings.Contains(out, "owner@example.com") {
		t.Fatalf("missing ACL entries: %q", out)
	}
}

func TestCalendarACL_JSONOutput(t *testing.T) {
	origNew := newCalendarService
	t.Cleanup(func() { newCalendarService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/calendars/test@example.com/acl") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{
					"id":   "user:owner@example.com",
					"role": "owner",
					"scope": map[string]any{
						"type":  "user",
						"value": "owner@example.com",
					},
				},
			},
		})
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
			if err := Execute([]string{"--output", "json", "--account", "a@b.com", "calendar", "acl", "test@example.com"}); err != nil {
				t.Fatalf("Execute: %v", err)
			}
		})
	})

	var parsed struct {
		Rules []struct {
			ID   string `json:"id"`
			Role string `json:"role"`
		} `json:"rules"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json parse: %v\nout=%q", err, out)
	}
	if len(parsed.Rules) != 1 || parsed.Rules[0].Role != "owner" {
		t.Fatalf("unexpected rules: %#v", parsed.Rules)
	}
}

func TestCalendarFreeBusy_Success(t *testing.T) {
	origNew := newCalendarService
	t.Cleanup(func() { newCalendarService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/freeBusy") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"calendars": map[string]any{
				"primary": map[string]any{
					"busy": []map[string]any{
						{
							"start": "2025-01-01T09:00:00Z",
							"end":   "2025-01-01T10:00:00Z",
						},
					},
				},
			},
		})
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
			if err := Execute([]string{"--output", "text", "--account", "a@b.com", "calendar", "freebusy", "primary", "--from", "2025-01-01T00:00:00Z", "--to", "2025-01-01T23:59:59Z"}); err != nil {
				t.Fatalf("Execute: %v", err)
			}
		})
	})

	if !strings.Contains(out, "CALENDAR") || !strings.Contains(out, "START") {
		t.Fatalf("unexpected output headers: %q", out)
	}
	if !strings.Contains(out, "primary") {
		t.Fatalf("missing calendar ID: %q", out)
	}
}

func TestCalendarFreeBusy_InvalidTimeRange(t *testing.T) {
	origNew := newCalendarService
	t.Cleanup(func() { newCalendarService = origNew })

	svc, err := calendar.NewService(context.Background(), option.WithoutAuthentication())
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	newCalendarService = func(context.Context, string) (*calendar.Service, error) { return svc, nil }

	// Test missing --from
	err = Execute([]string{"--output", "text", "--account", "a@b.com", "calendar", "freebusy", "primary", "--to", "2025-01-01T23:59:59Z"})
	if err == nil {
		t.Fatalf("expected error for missing --from")
	}
	if !strings.Contains(err.Error(), "required: --from and --to") {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test missing --to
	err = Execute([]string{"--output", "text", "--account", "a@b.com", "calendar", "freebusy", "primary", "--from", "2025-01-01T00:00:00Z"})
	if err == nil {
		t.Fatalf("expected error for missing --to")
	}

	// Test empty calendar IDs
	err = Execute([]string{"--output", "text", "--account", "a@b.com", "calendar", "freebusy", "  ,  ,  ", "--from", "2025-01-01T00:00:00Z", "--to", "2025-01-01T23:59:59Z"})
	if err == nil {
		t.Fatalf("expected error for empty calendar IDs")
	}
	if !strings.Contains(err.Error(), "no calendar IDs provided") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCalendarFreeBusy_JSONOutput(t *testing.T) {
	origNew := newCalendarService
	t.Cleanup(func() { newCalendarService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/freeBusy") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"calendars": map[string]any{
				"work@example.com": map[string]any{
					"busy": []map[string]any{
						{"start": "2025-01-01T09:00:00Z", "end": "2025-01-01T10:00:00Z"},
					},
				},
			},
		})
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
			if err := Execute([]string{"--output", "json", "--account", "a@b.com", "calendar", "freebusy", "work@example.com", "--from", "2025-01-01T00:00:00Z", "--to", "2025-01-01T23:59:59Z"}); err != nil {
				t.Fatalf("Execute: %v", err)
			}
		})
	})

	var parsed struct {
		Calendars map[string]struct {
			Busy []struct {
				Start string `json:"start"`
				End   string `json:"end"`
			} `json:"busy"`
		} `json:"calendars"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json parse: %v\nout=%q", err, out)
	}
	if _, ok := parsed.Calendars["work@example.com"]; !ok {
		t.Fatalf("missing calendar in response")
	}
}
