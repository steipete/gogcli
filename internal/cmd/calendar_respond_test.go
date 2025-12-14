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

func TestExecute_CalendarRespond_Accepted_JSON(t *testing.T) {
	origNew := newCalendarService
	t.Cleanup(func() { newCalendarService = origNew })

	var receivedPatch *calendar.Event

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/events/event1") && r.Method == http.MethodGet {
			// Return event with user as attendee
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":      "event1",
				"summary": "Meeting",
				"attendees": []map[string]any{
					{"email": "organizer@example.com", "organizer": true, "responseStatus": "accepted"},
					{"email": "a@b.com", "self": true, "responseStatus": "needsAction"},
				},
			})
			return
		}
		if strings.Contains(r.URL.Path, "/events/event1") && r.Method == http.MethodPatch {
			// Capture the patch request
			_ = json.NewDecoder(r.Body).Decode(&receivedPatch)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":      "event1",
				"summary": "Meeting",
				"attendees": []map[string]any{
					{"email": "organizer@example.com", "organizer": true, "responseStatus": "accepted"},
					{"email": "a@b.com", "self": true, "responseStatus": "accepted"},
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
			if err := Execute([]string{"--output", "json", "--account", "a@b.com", "calendar", "respond", "primary", "event1", "--status", "accepted"}); err != nil {
				t.Fatalf("Execute: %v", err)
			}
		})
	})

	var parsed struct {
		Event struct {
			ID        string `json:"id"`
			Summary   string `json:"summary"`
			Attendees []struct {
				Email          string `json:"email"`
				ResponseStatus string `json:"responseStatus"`
				Self           bool   `json:"self"`
			} `json:"attendees"`
		} `json:"event"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json parse: %v\nout=%q", err, out)
	}
	if parsed.Event.ID != "event1" {
		t.Fatalf("unexpected event ID: %q", parsed.Event.ID)
	}

	// Verify the attendee was updated
	var found bool
	for _, a := range parsed.Event.Attendees {
		if a.Email == "a@b.com" && a.Self {
			if a.ResponseStatus != "accepted" {
				t.Fatalf("unexpected response status: %q", a.ResponseStatus)
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("self attendee not found in response")
	}

	// Verify patch request was sent correctly
	if receivedPatch == nil {
		t.Fatalf("no patch request received")
	}
	if len(receivedPatch.Attendees) != 2 {
		t.Fatalf("unexpected attendees count in patch: %d", len(receivedPatch.Attendees))
	}
	for _, a := range receivedPatch.Attendees {
		if a.Email == "a@b.com" && a.ResponseStatus != "accepted" {
			t.Fatalf("patch did not update response status: %q", a.ResponseStatus)
		}
	}
}

func TestExecute_CalendarRespond_Declined_JSON(t *testing.T) {
	origNew := newCalendarService
	t.Cleanup(func() { newCalendarService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/events/event1") && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":      "event1",
				"summary": "Meeting",
				"attendees": []map[string]any{
					{"email": "organizer@example.com", "organizer": true, "responseStatus": "accepted"},
					{"email": "a@b.com", "self": true, "responseStatus": "needsAction"},
				},
			})
			return
		}
		if strings.Contains(r.URL.Path, "/events/event1") && r.Method == http.MethodPatch {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":      "event1",
				"summary": "Meeting",
				"attendees": []map[string]any{
					{"email": "organizer@example.com", "organizer": true, "responseStatus": "accepted"},
					{"email": "a@b.com", "self": true, "responseStatus": "declined"},
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
			if err := Execute([]string{"--output", "json", "--account", "a@b.com", "calendar", "respond", "primary", "event1", "--status", "declined"}); err != nil {
				t.Fatalf("Execute: %v", err)
			}
		})
	})

	var parsed struct {
		Event struct {
			Attendees []struct {
				Email          string `json:"email"`
				ResponseStatus string `json:"responseStatus"`
				Self           bool   `json:"self"`
			} `json:"attendees"`
		} `json:"event"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json parse: %v\nout=%q", err, out)
	}

	for _, a := range parsed.Event.Attendees {
		if a.Email == "a@b.com" && a.Self {
			if a.ResponseStatus != "declined" {
				t.Fatalf("unexpected response status: %q", a.ResponseStatus)
			}
			return
		}
	}
	t.Fatalf("self attendee not found")
}

func TestExecute_CalendarRespond_WithComment_JSON(t *testing.T) {
	origNew := newCalendarService
	t.Cleanup(func() { newCalendarService = origNew })

	var receivedPatch *calendar.Event

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/events/event1") && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":      "event1",
				"summary": "Meeting",
				"attendees": []map[string]any{
					{"email": "organizer@example.com", "organizer": true, "responseStatus": "accepted"},
					{"email": "a@b.com", "self": true, "responseStatus": "needsAction"},
				},
			})
			return
		}
		if strings.Contains(r.URL.Path, "/events/event1") && r.Method == http.MethodPatch {
			_ = json.NewDecoder(r.Body).Decode(&receivedPatch)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":      "event1",
				"summary": "Meeting",
				"attendees": []map[string]any{
					{"email": "organizer@example.com", "organizer": true, "responseStatus": "accepted"},
					{"email": "a@b.com", "self": true, "responseStatus": "tentative", "comment": "Maybe I can join"},
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
			if err := Execute([]string{"--output", "json", "--account", "a@b.com", "calendar", "respond", "primary", "event1", "--status", "tentative", "--comment", "Maybe I can join"}); err != nil {
				t.Fatalf("Execute: %v", err)
			}
		})
	})

	var parsed struct {
		Event struct {
			Attendees []struct {
				Email          string `json:"email"`
				ResponseStatus string `json:"responseStatus"`
				Comment        string `json:"comment"`
				Self           bool   `json:"self"`
			} `json:"attendees"`
		} `json:"event"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json parse: %v\nout=%q", err, out)
	}

	for _, a := range parsed.Event.Attendees {
		if a.Email == "a@b.com" && a.Self {
			if a.ResponseStatus != "tentative" {
				t.Fatalf("unexpected response status: %q", a.ResponseStatus)
			}
			if a.Comment != "Maybe I can join" {
				t.Fatalf("unexpected comment: %q", a.Comment)
			}
			return
		}
	}
	t.Fatalf("self attendee not found")

	// Verify comment was sent in patch
	if receivedPatch == nil {
		t.Fatalf("no patch request received")
	}
	for _, a := range receivedPatch.Attendees {
		if a.Email == "a@b.com" && a.Comment != "Maybe I can join" {
			t.Fatalf("patch did not include comment: %q", a.Comment)
		}
	}
}

func TestExecute_CalendarRespond_NotAttendee(t *testing.T) {
	origNew := newCalendarService
	t.Cleanup(func() { newCalendarService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/events/event1") && r.Method == http.MethodGet {
			// Return event without user as attendee
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":      "event1",
				"summary": "Meeting",
				"attendees": []map[string]any{
					{"email": "organizer@example.com", "organizer": true, "responseStatus": "accepted"},
					{"email": "other@example.com", "responseStatus": "needsAction"},
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

	_ = captureStdout(t, func() {
		_ = captureStderr(t, func() {
			err := Execute([]string{"--output", "json", "--account", "a@b.com", "calendar", "respond", "primary", "event1", "--status", "accepted"})
			if err == nil {
				t.Fatalf("expected error for non-attendee")
			}
			if !strings.Contains(err.Error(), "not an attendee") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	})
}

func TestExecute_CalendarRespond_IsOrganizer(t *testing.T) {
	origNew := newCalendarService
	t.Cleanup(func() { newCalendarService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/events/event1") && r.Method == http.MethodGet {
			// Return event with user as organizer
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":      "event1",
				"summary": "Meeting",
				"attendees": []map[string]any{
					{"email": "a@b.com", "self": true, "organizer": true, "responseStatus": "accepted"},
					{"email": "other@example.com", "responseStatus": "needsAction"},
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

	_ = captureStdout(t, func() {
		_ = captureStderr(t, func() {
			err := Execute([]string{"--output", "json", "--account", "a@b.com", "calendar", "respond", "primary", "event1", "--status", "accepted"})
			if err == nil {
				t.Fatalf("expected error for organizer")
			}
			if !strings.Contains(err.Error(), "organizer") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	})
}

func TestExecute_CalendarRespond_InvalidStatus(t *testing.T) {
	origNew := newCalendarService
	t.Cleanup(func() { newCalendarService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			err := Execute([]string{"--output", "json", "--account", "a@b.com", "calendar", "respond", "primary", "event1", "--status", "invalid"})
			if err == nil {
				t.Fatalf("expected error for invalid status")
			}
			if !strings.Contains(err.Error(), "invalid") || !strings.Contains(err.Error(), "status") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	})
}

func TestExecute_CalendarRespond_TableOutput(t *testing.T) {
	origNew := newCalendarService
	t.Cleanup(func() { newCalendarService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/events/event1") && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":      "event1",
				"summary": "Meeting",
				"attendees": []map[string]any{
					{"email": "organizer@example.com", "organizer": true, "responseStatus": "accepted"},
					{"email": "a@b.com", "self": true, "responseStatus": "needsAction"},
				},
			})
			return
		}
		if strings.Contains(r.URL.Path, "/events/event1") && r.Method == http.MethodPatch {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":       "event1",
				"summary":  "Meeting",
				"htmlLink": "https://calendar.google.com/event?eid=abc",
				"attendees": []map[string]any{
					{"email": "organizer@example.com", "organizer": true, "responseStatus": "accepted"},
					{"email": "a@b.com", "self": true, "responseStatus": "accepted"},
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
			if err := Execute([]string{"--account", "a@b.com", "calendar", "respond", "primary", "event1", "--status", "accepted"}); err != nil {
				t.Fatalf("Execute: %v", err)
			}
		})
	})

	// Verify table output contains expected fields
	if !strings.Contains(out, "event1") {
		t.Fatalf("output missing event id: %q", out)
	}
	if !strings.Contains(out, "accepted") {
		t.Fatalf("output missing response status: %q", out)
	}
	if !strings.Contains(out, "https://calendar.google.com/event?eid=abc") {
		t.Fatalf("output missing link: %q", out)
	}
}
