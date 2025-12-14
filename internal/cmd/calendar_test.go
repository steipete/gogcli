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

func TestParseEventTimes(t *testing.T) {
	// Test timed event
	timedEvent := &calendar.Event{
		Start: &calendar.EventDateTime{DateTime: "2025-01-15T10:00:00Z"},
		End:   &calendar.EventDateTime{DateTime: "2025-01-15T11:00:00Z"},
	}
	start, end := parseEventTimes(timedEvent)
	if start.IsZero() || end.IsZero() {
		t.Fatalf("expected non-zero times for timed event")
	}
	if start.Hour() != 10 || end.Hour() != 11 {
		t.Fatalf("unexpected hours: start=%d, end=%d", start.Hour(), end.Hour())
	}

	// Test all-day event
	allDayEvent := &calendar.Event{
		Start: &calendar.EventDateTime{Date: "2025-01-15"},
		End:   &calendar.EventDateTime{Date: "2025-01-16"},
	}
	start2, end2 := parseEventTimes(allDayEvent)
	if start2.IsZero() || end2.IsZero() {
		t.Fatalf("expected non-zero times for all-day event")
	}
	if start2.Day() != 15 || end2.Day() != 16 {
		t.Fatalf("unexpected days: start=%d, end=%d", start2.Day(), end2.Day())
	}

	// Test nil event
	start3, end3 := parseEventTimes(nil)
	if !start3.IsZero() || !end3.IsZero() {
		t.Fatalf("expected zero times for nil event")
	}

	// Test event with nil Start/End
	partialEvent := &calendar.Event{}
	start4, end4 := parseEventTimes(partialEvent)
	if !start4.IsZero() || !end4.IsZero() {
		t.Fatalf("expected zero times for event with nil Start/End")
	}
}

