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
