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

func TestExecute_CalendarCalendars_JSON(t *testing.T) {
	origNew := newCalendarService
	t.Cleanup(func() { newCalendarService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !(strings.Contains(r.URL.Path, "calendarList") && r.Method == http.MethodGet) {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{"id": "c1", "summary": "One", "accessRole": "owner"},
				{"id": "c2", "summary": "Two", "accessRole": "reader"},
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
			if err := Execute([]string{"--output", "json", "--account", "a@b.com", "calendar", "calendars"}); err != nil {
				t.Fatalf("Execute: %v", err)
			}
		})
	})

	var parsed struct {
		Calendars []struct {
			ID         string `json:"id"`
			Summary    string `json:"summary"`
			AccessRole string `json:"accessRole"`
		} `json:"calendars"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json parse: %v\nout=%q", err, out)
	}
	if len(parsed.Calendars) != 2 || parsed.Calendars[0].ID != "c1" || parsed.Calendars[1].ID != "c2" {
		t.Fatalf("unexpected calendars: %#v", parsed.Calendars)
	}
}

func TestCalendarCreateCmd_WithColor(t *testing.T) {
	origNew := newCalendarService
	t.Cleanup(func() { newCalendarService = origNew })

	var receivedEvent *calendar.Event
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "events") && r.Method == http.MethodPost {
			var evt calendar.Event
			if err := json.NewDecoder(r.Body).Decode(&evt); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			receivedEvent = &evt
			evt.Id = "test-event-id"
			evt.HtmlLink = "https://calendar.google.com/event"
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(evt)
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
			args := []string{
				"--output", "json",
				"--account", "test@example.com",
				"calendar", "create", "primary",
				"--summary", "Test Event",
				"--start", "2025-01-15T10:00:00Z",
				"--end", "2025-01-15T11:00:00Z",
				"--color", "5",
			}
			if err := Execute(args); err != nil {
				t.Fatalf("Execute: %v", err)
			}
		})
	})

	if receivedEvent == nil {
		t.Fatal("no event received by mock server")
	}
	if receivedEvent.ColorId != "5" {
		t.Errorf("expected ColorId=5, got %q", receivedEvent.ColorId)
	}
	if receivedEvent.Summary != "Test Event" {
		t.Errorf("expected Summary='Test Event', got %q", receivedEvent.Summary)
	}

	var parsed struct {
		Event struct {
			ID      string `json:"id"`
			ColorId string `json:"colorId"`
		} `json:"event"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json parse: %v\nout=%q", err, out)
	}
	if parsed.Event.ColorId != "5" {
		t.Errorf("response ColorId=%q, want '5'", parsed.Event.ColorId)
	}
}

func TestCalendarCreateCmd_WithOrganizer(t *testing.T) {
	origNew := newCalendarService
	t.Cleanup(func() { newCalendarService = origNew })

	var receivedEvent *calendar.Event
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "events") && r.Method == http.MethodPost {
			var evt calendar.Event
			if err := json.NewDecoder(r.Body).Decode(&evt); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			receivedEvent = &evt
			evt.Id = "test-event-id"
			evt.HtmlLink = "https://calendar.google.com/event"
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(evt)
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

	_ = captureStdout(t, func() {
		_ = captureStderr(t, func() {
			args := []string{
				"--output", "json",
				"--account", "test@example.com",
				"calendar", "create", "primary",
				"--summary", "Team Meeting",
				"--start", "2025-01-15T14:00:00Z",
				"--end", "2025-01-15T15:00:00Z",
				"--organizer", "organizer@example.com",
			}
			if err := Execute(args); err != nil {
				t.Fatalf("Execute: %v", err)
			}
		})
	})

	if receivedEvent == nil {
		t.Fatal("no event received by mock server")
	}
	if receivedEvent.Organizer == nil {
		t.Fatal("expected Organizer to be set")
	}
	if receivedEvent.Organizer.Email != "organizer@example.com" {
		t.Errorf("expected Organizer.Email='organizer@example.com', got %q", receivedEvent.Organizer.Email)
	}
	if receivedEvent.Summary != "Team Meeting" {
		t.Errorf("expected Summary='Team Meeting', got %q", receivedEvent.Summary)
	}
}

func TestCalendarCreateCmd_WithColorAndOrganizer(t *testing.T) {
	origNew := newCalendarService
	t.Cleanup(func() { newCalendarService = origNew })

	var receivedEvent *calendar.Event
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "events") && r.Method == http.MethodPost {
			var evt calendar.Event
			if err := json.NewDecoder(r.Body).Decode(&evt); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			receivedEvent = &evt
			evt.Id = "test-event-id"
			evt.HtmlLink = "https://calendar.google.com/event"
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(evt)
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
			args := []string{
				"--output", "json",
				"--account", "test@example.com",
				"calendar", "create", "primary",
				"--summary", "Important Meeting",
				"--start", "2025-01-16T09:00:00Z",
				"--end", "2025-01-16T10:00:00Z",
				"--organizer", "boss@example.com",
				"--color", "11",
			}
			if err := Execute(args); err != nil {
				t.Fatalf("Execute: %v", err)
			}
		})
	})

	if receivedEvent == nil {
		t.Fatal("no event received by mock server")
	}
	if receivedEvent.ColorId != "11" {
		t.Errorf("expected ColorId='11', got %q", receivedEvent.ColorId)
	}
	if receivedEvent.Organizer == nil {
		t.Fatal("expected Organizer to be set")
	}
	if receivedEvent.Organizer.Email != "boss@example.com" {
		t.Errorf("expected Organizer.Email='boss@example.com', got %q", receivedEvent.Organizer.Email)
	}
	if receivedEvent.Summary != "Important Meeting" {
		t.Errorf("expected Summary='Important Meeting', got %q", receivedEvent.Summary)
	}

	var parsed struct {
		Event struct {
			ID      string `json:"id"`
			ColorId string `json:"colorId"`
		} `json:"event"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json parse: %v\nout=%q", err, out)
	}
	if parsed.Event.ColorId != "11" {
		t.Errorf("response ColorId=%q, want '11'", parsed.Event.ColorId)
	}
}
