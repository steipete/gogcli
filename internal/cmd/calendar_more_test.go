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

func TestBuildAttendees(t *testing.T) {
	if got := buildAttendees(""); got != nil {
		t.Fatalf("unexpected: %#v", got)
	}
	got := buildAttendees(" a@b.com, c@d.com ")
	if len(got) != 2 || got[0].Email != "a@b.com" || got[1].Email != "c@d.com" {
		t.Fatalf("unexpected: %#v", got)
	}
}

func TestEventStartEnd(t *testing.T) {
	e := &calendar.Event{
		Start: &calendar.EventDateTime{DateTime: "2025-12-12T10:00:00Z"},
		End:   &calendar.EventDateTime{DateTime: "2025-12-12T11:00:00Z"},
	}
	if got := eventStart(e); got != "2025-12-12T10:00:00Z" {
		t.Fatalf("unexpected: %q", got)
	}
	if got := eventEnd(e); got != "2025-12-12T11:00:00Z" {
		t.Fatalf("unexpected: %q", got)
	}

	allDay := &calendar.Event{
		Start: &calendar.EventDateTime{Date: "2025-12-12"},
		End:   &calendar.EventDateTime{Date: "2025-12-13"},
	}
	if got := eventStart(allDay); got != "2025-12-12" {
		t.Fatalf("unexpected: %q", got)
	}
	if got := eventEnd(allDay); got != "2025-12-13" {
		t.Fatalf("unexpected: %q", got)
	}
}

func TestOrEmpty(t *testing.T) {
	if got := orEmpty("", "x"); got != "x" {
		t.Fatalf("unexpected: %q", got)
	}
	if got := orEmpty("  ", "x"); got != "x" {
		t.Fatalf("unexpected: %q", got)
	}
	if got := orEmpty("y", "x"); got != "y" {
		t.Fatalf("unexpected: %q", got)
	}
}

func TestCalendarCreate_AllDayEvent(t *testing.T) {
	origNew := newCalendarService
	t.Cleanup(func() { newCalendarService = origNew })

	var receivedEvent *calendar.Event

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/calendars/primary/events") {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "expected POST", http.StatusMethodNotAllowed)
			return
		}

		var event calendar.Event
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		receivedEvent = &event

		event.Id = "event123"
		event.HtmlLink = "https://calendar.google.com/event?eid=event123"
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(event)
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
			err := Execute([]string{
				"--output", "json",
				"--account", "test@example.com",
				"calendar", "create", "primary",
				"--summary", "Team Meeting",
				"--start", "2025-12-15",
				"--end", "2025-12-16",
				"--all-day",
			})
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
		})
	})

	if receivedEvent == nil {
		t.Fatal("no event received by server")
	}
	if receivedEvent.Summary != "Team Meeting" {
		t.Fatalf("unexpected summary: %q", receivedEvent.Summary)
	}
	if receivedEvent.Start == nil || receivedEvent.Start.Date != "2025-12-15" {
		t.Fatalf("unexpected start: %#v", receivedEvent.Start)
	}

	var parsed struct {
		Event struct {
			ID string `json:"id"`
		} `json:"event"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json parse: %v\nout=%q", err, out)
	}
	if parsed.Event.ID != "event123" {
		t.Fatalf("unexpected event ID: %q", parsed.Event.ID)
	}
}

func TestCalendarCreate_WithAttendees(t *testing.T) {
	origNew := newCalendarService
	t.Cleanup(func() { newCalendarService = origNew })

	var receivedEvent *calendar.Event

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/calendars/primary/events") {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "expected POST", http.StatusMethodNotAllowed)
			return
		}

		var event calendar.Event
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		receivedEvent = &event

		event.Id = "event456"
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(event)
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
			err := Execute([]string{
				"--output", "json",
				"--account", "test@example.com",
				"calendar", "create", "primary",
				"--summary", "Project Sync",
				"--start", "2025-12-15T10:00:00Z",
				"--end", "2025-12-15T11:00:00Z",
				"--attendees", "alice@example.com, bob@example.com",
			})
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
		})
	})

	if receivedEvent == nil {
		t.Fatal("no event received by server")
	}
	if len(receivedEvent.Attendees) != 2 {
		t.Fatalf("expected 2 attendees, got %d", len(receivedEvent.Attendees))
	}
	if receivedEvent.Attendees[0].Email != "alice@example.com" {
		t.Fatalf("unexpected first attendee: %q", receivedEvent.Attendees[0].Email)
	}
}

func TestCalendarCreate_MissingRequiredFields(t *testing.T) {
	origNew := newCalendarService
	t.Cleanup(func() { newCalendarService = origNew })

	svc, err := calendar.NewService(context.Background(), option.WithoutAuthentication())
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	newCalendarService = func(context.Context, string) (*calendar.Service, error) { return svc, nil }

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing summary",
			args: []string{
				"--account", "test@example.com",
				"calendar", "create", "primary",
				"--start", "2025-12-15T10:00:00Z",
				"--end", "2025-12-15T11:00:00Z",
			},
		},
		{
			name: "missing start",
			args: []string{
				"--account", "test@example.com",
				"calendar", "create", "primary",
				"--summary", "Meeting",
				"--end", "2025-12-15T11:00:00Z",
			},
		},
		{
			name: "missing end",
			args: []string{
				"--account", "test@example.com",
				"calendar", "create", "primary",
				"--summary", "Meeting",
				"--start", "2025-12-15T10:00:00Z",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Execute(tt.args)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), "required: --summary, --start, --end") {
				t.Fatalf("expected required fields error, got: %v", err)
			}
		})
	}
}

func TestCalendarUpdate_Success(t *testing.T) {
	origNew := newCalendarService
	t.Cleanup(func() { newCalendarService = origNew })

	existingEvent := &calendar.Event{
		Id:      "event789",
		Summary: "Old Title",
		Start:   &calendar.EventDateTime{DateTime: "2025-12-15T10:00:00Z"},
		End:     &calendar.EventDateTime{DateTime: "2025-12-15T11:00:00Z"},
	}

	var updatedEvent *calendar.Event

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/calendars/primary/events/event789") {
			if r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(existingEvent)
				return
			}
			if r.Method == http.MethodPut {
				var event calendar.Event
				if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				updatedEvent = &event
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(event)
				return
			}
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
			err := Execute([]string{
				"--output", "json",
				"--account", "test@example.com",
				"calendar", "update", "primary", "event789",
				"--summary", "New Title",
			})
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
		})
	})

	if updatedEvent == nil {
		t.Fatal("no updated event received by server")
	}
	if updatedEvent.Summary != "New Title" {
		t.Fatalf("expected 'New Title', got %q", updatedEvent.Summary)
	}
}

func TestCalendarUpdate_NoChangesProvided(t *testing.T) {
	origNew := newCalendarService
	t.Cleanup(func() { newCalendarService = origNew })

	existingEvent := &calendar.Event{
		Id:      "event789",
		Summary: "Meeting",
		Start:   &calendar.EventDateTime{DateTime: "2025-12-15T10:00:00Z"},
		End:     &calendar.EventDateTime{DateTime: "2025-12-15T11:00:00Z"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/calendars/primary/events/event789") && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(existingEvent)
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

	err = Execute([]string{
		"--output", "json",
		"--account", "test@example.com",
		"calendar", "update", "primary", "event789",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no updates provided") {
		t.Fatalf("expected 'no updates provided' error, got: %v", err)
	}
}
