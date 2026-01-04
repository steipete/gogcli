package cmd

import (
	"testing"

	"google.golang.org/api/calendar/v3"
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

func TestBuildRecurrence(t *testing.T) {
	// nil input
	if got := buildRecurrence(nil); got != nil {
		t.Fatalf("expected nil, got %#v", got)
	}

	// empty slice
	if got := buildRecurrence([]string{}); got != nil {
		t.Fatalf("expected nil, got %#v", got)
	}

	// only empty strings
	if got := buildRecurrence([]string{"", "  "}); got != nil {
		t.Fatalf("expected nil, got %#v", got)
	}

	// valid rules
	got := buildRecurrence([]string{"RRULE:FREQ=DAILY", "", "EXDATE:20250101"})
	if len(got) != 2 || got[0] != "RRULE:FREQ=DAILY" || got[1] != "EXDATE:20250101" {
		t.Fatalf("unexpected: %#v", got)
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input   string
		want    int64
		wantErr bool
	}{
		// raw minutes
		{"30", 30, false},
		{"0", 0, false},
		{"40320", 40320, false}, // max allowed

		// with suffixes
		{"30m", 30, false},
		{"1h", 60, false},
		{"2h", 120, false},
		{"1d", 1440, false},
		{"3d", 4320, false},
		{"1w", 10080, false},
		{"4w", 40320, false}, // max allowed

		// case insensitive
		{"1H", 60, false},
		{"1D", 1440, false},
		{"1M", 1, false},
		{"30M", 30, false},

		// errors
		{"", 0, true},
		{"abc", 0, true},
		{"-1", 0, true},
		{"40321", 0, true}, // over max
		{"5w", 0, true},    // 5 weeks > 4 weeks max
		{"1x", 0, true},    // invalid unit
	}

	for _, tc := range tests {
		got, err := parseDuration(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseDuration(%q): expected error, got %d", tc.input, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseDuration(%q): unexpected error: %v", tc.input, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseDuration(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestParseReminder(t *testing.T) {
	tests := []struct {
		input       string
		wantMethod  string
		wantMinutes int64
		wantErr     bool
	}{
		{"popup:30m", "popup", 30, false},
		{"email:1h", "email", 60, false},
		{"POPUP:1d", "popup", 1440, false},
		{"EMAIL:3d", "email", 4320, false},
		{"popup:60", "popup", 60, false}, // raw minutes

		// errors
		{"", "", 0, true},
		{"popup", "", 0, true},       // no colon
		{"sms:30m", "", 0, true},     // invalid method
		{"popup:abc", "", 0, true},   // invalid duration
		{"popup:-1", "", 0, true},    // negative duration
		{"popup:50000", "", 0, true}, // over max
	}

	for _, tc := range tests {
		method, minutes, err := parseReminder(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseReminder(%q): expected error, got method=%q minutes=%d", tc.input, method, minutes)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseReminder(%q): unexpected error: %v", tc.input, err)
			continue
		}
		if method != tc.wantMethod || minutes != tc.wantMinutes {
			t.Errorf("parseReminder(%q) = (%q, %d), want (%q, %d)", tc.input, method, minutes, tc.wantMethod, tc.wantMinutes)
		}
	}
}

func TestBuildReminders(t *testing.T) {
	// nil input - returns nil (use calendar defaults)
	got, err := buildReminders(nil)
	if err != nil || got != nil {
		t.Fatalf("expected (nil, nil), got (%#v, %v)", got, err)
	}

	// empty slice - returns nil
	got, err = buildReminders([]string{})
	if err != nil || got != nil {
		t.Fatalf("expected (nil, nil), got (%#v, %v)", got, err)
	}

	// only empty strings - returns nil
	got, err = buildReminders([]string{"", "  "})
	if err != nil || got != nil {
		t.Fatalf("expected (nil, nil), got (%#v, %v)", got, err)
	}

	// valid reminders
	got, err = buildReminders([]string{"popup:30m", "email:1d"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || got.UseDefault || len(got.Overrides) != 2 {
		t.Fatalf("unexpected: %#v", got)
	}
	if got.Overrides[0].Method != "popup" || got.Overrides[0].Minutes != 30 {
		t.Fatalf("unexpected override[0]: %#v", got.Overrides[0])
	}
	if got.Overrides[1].Method != "email" || got.Overrides[1].Minutes != 1440 {
		t.Fatalf("unexpected override[1]: %#v", got.Overrides[1])
	}

	// too many reminders (max 5)
	_, err = buildReminders([]string{"popup:1m", "popup:2m", "popup:3m", "popup:4m", "popup:5m", "popup:6m"})
	if err == nil {
		t.Fatalf("expected error for >5 reminders")
	}

	// invalid reminder
	_, err = buildReminders([]string{"popup:30m", "invalid"})
	if err == nil {
		t.Fatalf("expected error for invalid reminder")
	}
}
