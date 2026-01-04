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

func TestBuildColorId(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"1", "1", false},
		{"11", "11", false},
		{"0", "", true},
		{"12", "", true},
		{"", "", false},
		{"abc", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := validateColorId(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateColorId(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("validateColorId(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateVisibility(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"default", "default", false},
		{"public", "public", false},
		{"private", "private", false},
		{"confidential", "confidential", false},
		{"DEFAULT", "default", false},
		{"", "", false},
		{"invalid", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := validateVisibility(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateVisibility(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("validateVisibility(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateTransparency(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"opaque", "opaque", false},
		{"transparent", "transparent", false},
		{"busy", "opaque", false},
		{"free", "transparent", false},
		{"OPAQUE", "opaque", false},
		{"", "", false},
		{"invalid", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := validateTransparency(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTransparency(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("validateTransparency(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateSendUpdates(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"all", "all", false},
		{"externalOnly", "externalOnly", false},
		{"none", "none", false},
		{"ALL", "all", false},
		{"", "", false},
		{"invalid", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := validateSendUpdates(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSendUpdates(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("validateSendUpdates(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseAttendee(t *testing.T) {
	tests := []struct {
		input    string
		email    string
		optional bool
		comment  string
		isNil    bool
	}{
		{"alice@example.com", "alice@example.com", false, "", false},
		{"bob@example.com;optional", "bob@example.com", true, "", false},
		{"carol@example.com;comment=FYI only", "carol@example.com", false, "FYI only", false},
		{"dave@example.com;OPTIONAL;comment=Hi", "dave@example.com", true, "Hi", false},
		{";optional", "", false, "", true},
		{"", "", false, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseAttendee(tt.input)
			if tt.isNil {
				if got != nil {
					t.Fatalf("expected nil attendee, got %#v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected attendee, got nil")
			}
			if got.Email != tt.email || got.Optional != tt.optional || got.Comment != tt.comment {
				t.Fatalf("unexpected attendee: %#v", got)
			}
		})
	}
}

func TestRecurrenceUntil(t *testing.T) {
	got, err := recurrenceUntil("2025-01-10")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "20250109" {
		t.Fatalf("unexpected until date: %s", got)
	}

	got, err = recurrenceUntil("2025-01-10T12:00:00Z")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "20250110T115959Z" {
		t.Fatalf("unexpected until datetime: %s", got)
	}
}

func TestTruncateRecurrence(t *testing.T) {
	rules := []string{
		"RRULE:FREQ=WEEKLY;COUNT=10",
		"EXDATE:20250101T100000Z",
	}
	truncated, err := truncateRecurrence(rules, "2025-01-10T12:00:00Z")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(truncated) != 2 {
		t.Fatalf("unexpected rule count: %#v", truncated)
	}
	if truncated[0] != "RRULE:FREQ=WEEKLY;UNTIL=20250110T115959Z" {
		t.Fatalf("unexpected RRULE: %s", truncated[0])
	}
	if truncated[1] != "EXDATE:20250101T100000Z" {
		t.Fatalf("unexpected EXDATE: %s", truncated[1])
	}
}
