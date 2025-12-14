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

func TestCalendarConflictsCmd_WithConflicts_JSON(t *testing.T) {
	origNew := newCalendarService
	t.Cleanup(func() { newCalendarService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/freeBusy") && r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"calendars": map[string]any{
					"primary": map[string]any{
						"busy": []map[string]any{
							{
								"start": "2024-12-13T10:00:00Z",
								"end":   "2024-12-13T11:00:00Z",
							},
						},
					},
					"work@example.com": map[string]any{
						"busy": []map[string]any{
							{
								"start": "2024-12-13T10:30:00Z",
								"end":   "2024-12-13T11:30:00Z",
							},
						},
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
			if err := Execute([]string{
				"--output", "json",
				"--account", "a@b.com",
				"calendar", "conflicts",
				"--from", "2024-12-13T09:00:00Z",
				"--to", "2024-12-13T12:00:00Z",
				"--calendars", "primary,work@example.com",
			}); err != nil {
				t.Fatalf("Execute: %v", err)
			}
		})
	})

	var parsed struct {
		Conflicts []struct {
			Start     string   `json:"start"`
			End       string   `json:"end"`
			Calendars []string `json:"calendars"`
		} `json:"conflicts"`
		Count int `json:"count"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json parse: %v\nout=%q", err, out)
	}
	if parsed.Count != 1 {
		t.Errorf("expected count 1, got %d", parsed.Count)
	}
	if len(parsed.Conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(parsed.Conflicts))
	}
	// Overlap is from 10:30 to 11:00
	if parsed.Conflicts[0].Start != "2024-12-13T10:30:00Z" {
		t.Errorf("unexpected conflict start: %q", parsed.Conflicts[0].Start)
	}
	if parsed.Conflicts[0].End != "2024-12-13T11:00:00Z" {
		t.Errorf("unexpected conflict end: %q", parsed.Conflicts[0].End)
	}
	if len(parsed.Conflicts[0].Calendars) != 2 {
		t.Fatalf("expected 2 calendars in conflict, got %d", len(parsed.Conflicts[0].Calendars))
	}
}

func TestCalendarConflictsCmd_NoConflicts_JSON(t *testing.T) {
	origNew := newCalendarService
	t.Cleanup(func() { newCalendarService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/freeBusy") && r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"calendars": map[string]any{
					"primary": map[string]any{
						"busy": []map[string]any{
							{
								"start": "2024-12-13T10:00:00Z",
								"end":   "2024-12-13T11:00:00Z",
							},
						},
					},
					"work@example.com": map[string]any{
						"busy": []map[string]any{
							{
								"start": "2024-12-13T12:00:00Z",
								"end":   "2024-12-13T13:00:00Z",
							},
						},
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
			if err := Execute([]string{
				"--output", "json",
				"--account", "a@b.com",
				"calendar", "conflicts",
				"--from", "2024-12-13T09:00:00Z",
				"--to", "2024-12-13T14:00:00Z",
				"--calendars", "primary,work@example.com",
			}); err != nil {
				t.Fatalf("Execute: %v", err)
			}
		})
	})

	var parsed struct {
		Conflicts []map[string]any `json:"conflicts"`
		Count     int              `json:"count"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json parse: %v\nout=%q", err, out)
	}
	if parsed.Count != 0 {
		t.Errorf("expected count 0, got %d", parsed.Count)
	}
	if len(parsed.Conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d", len(parsed.Conflicts))
	}
}

func TestCalendarConflictsCmd_TableOutput(t *testing.T) {
	origNew := newCalendarService
	t.Cleanup(func() { newCalendarService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/freeBusy") && r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"calendars": map[string]any{
					"primary": map[string]any{
						"busy": []map[string]any{
							{
								"start": "2024-12-13T10:00:00Z",
								"end":   "2024-12-13T11:00:00Z",
							},
						},
					},
					"work@example.com": map[string]any{
						"busy": []map[string]any{
							{
								"start": "2024-12-13T10:30:00Z",
								"end":   "2024-12-13T11:30:00Z",
							},
						},
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
			if err := Execute([]string{
				"--account", "a@b.com",
				"calendar", "conflicts",
				"--from", "2024-12-13T09:00:00Z",
				"--to", "2024-12-13T12:00:00Z",
				"--calendars", "primary,work@example.com",
			}); err != nil {
				t.Fatalf("Execute: %v", err)
			}
		})
	})

	// Verify table output contains expected elements
	if !strings.Contains(out, "CONFLICTS FOUND: 1") {
		t.Errorf("output missing conflict count: %q", out)
	}
	if !strings.Contains(out, "2024-12-13T10:30:00Z") {
		t.Errorf("output missing conflict start time: %q", out)
	}
	if !strings.Contains(out, "2024-12-13T11:00:00Z") {
		t.Errorf("output missing conflict end time: %q", out)
	}
	if !strings.Contains(out, "primary") || !strings.Contains(out, "work@example.com") {
		t.Errorf("output missing calendar IDs: %q", out)
	}
	if !strings.Contains(out, "START") || !strings.Contains(out, "END") || !strings.Contains(out, "CALENDARS") {
		t.Errorf("output missing table headers: %q", out)
	}
}

func TestCalendarConflictsCmd_MultiCalendar(t *testing.T) {
	origNew := newCalendarService
	t.Cleanup(func() { newCalendarService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/freeBusy") && r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"calendars": map[string]any{
					"primary": map[string]any{
						"busy": []map[string]any{
							{
								"start": "2024-12-13T10:00:00Z",
								"end":   "2024-12-13T11:00:00Z",
							},
						},
					},
					"work@example.com": map[string]any{
						"busy": []map[string]any{
							{
								"start": "2024-12-13T10:30:00Z",
								"end":   "2024-12-13T11:30:00Z",
							},
						},
					},
					"personal@example.com": map[string]any{
						"busy": []map[string]any{
							{
								"start": "2024-12-13T10:45:00Z",
								"end":   "2024-12-13T11:15:00Z",
							},
						},
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
			if err := Execute([]string{
				"--output", "json",
				"--account", "a@b.com",
				"calendar", "conflicts",
				"--from", "2024-12-13T09:00:00Z",
				"--to", "2024-12-13T12:00:00Z",
				"--calendars", "primary,work@example.com,personal@example.com",
			}); err != nil {
				t.Fatalf("Execute: %v", err)
			}
		})
	})

	var parsed struct {
		Conflicts []struct {
			Start     string   `json:"start"`
			End       string   `json:"end"`
			Calendars []string `json:"calendars"`
		} `json:"conflicts"`
		Count int `json:"count"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json parse: %v\nout=%q", err, out)
	}

	// Should have 3 conflicts:
	// 1. primary vs work (10:30-11:00)
	// 2. primary vs personal (10:45-11:00)
	// 3. work vs personal (10:45-11:15)
	if parsed.Count != 3 {
		t.Errorf("expected count 3, got %d", parsed.Count)
	}
	if len(parsed.Conflicts) != 3 {
		t.Fatalf("expected 3 conflicts, got %d", len(parsed.Conflicts))
	}

	// Verify all conflicts have exactly 2 calendars involved
	for i, c := range parsed.Conflicts {
		if len(c.Calendars) != 2 {
			t.Errorf("conflict %d: expected 2 calendars, got %d", i, len(c.Calendars))
		}
	}
}

func TestCalendarConflictsCmd_NoConflicts_TableOutput(t *testing.T) {
	origNew := newCalendarService
	t.Cleanup(func() { newCalendarService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/freeBusy") && r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"calendars": map[string]any{
					"primary": map[string]any{
						"busy": []map[string]any{},
					},
					"work@example.com": map[string]any{
						"busy": []map[string]any{},
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
			if err := Execute([]string{
				"--account", "a@b.com",
				"calendar", "conflicts",
				"--from", "2024-12-13T09:00:00Z",
				"--to", "2024-12-13T14:00:00Z",
				"--calendars", "primary,work@example.com",
			}); err != nil {
				t.Fatalf("Execute: %v", err)
			}
		})
	})

	if !strings.Contains(out, "No conflicts found") {
		t.Errorf("expected 'No conflicts found' message, got: %q", out)
	}
}
