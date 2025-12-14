package cmd

import (
	"strings"
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

func TestSortEventsByStartTime(t *testing.T) {
	events := []*eventWithCalendar{
		{
			Event: &calendar.Event{
				Id:      "3",
				Start:   &calendar.EventDateTime{DateTime: "2025-01-03T10:00:00Z"},
				Summary: "Third",
			},
			CalendarID: "cal1",
		},
		{
			Event: &calendar.Event{
				Id:      "1",
				Start:   &calendar.EventDateTime{DateTime: "2025-01-01T10:00:00Z"},
				Summary: "First",
			},
			CalendarID: "cal2",
		},
		{
			Event: &calendar.Event{
				Id:      "2",
				Start:   &calendar.EventDateTime{DateTime: "2025-01-02T10:00:00Z"},
				Summary: "Second",
			},
			CalendarID: "cal1",
		},
	}

	sortEventsByStartTime(events)

	if events[0].Id != "1" || events[1].Id != "2" || events[2].Id != "3" {
		t.Fatalf("events not sorted correctly: got IDs %s, %s, %s", events[0].Id, events[1].Id, events[2].Id)
	}
}

func TestCalendarEventsCmd_RequiresCalendarIdOrAll(t *testing.T) {
	flags := &rootFlags{Account: "test@example.com"}
	cmd := newCalendarEventsCmd(flags)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "calendarId required unless --all is specified") {
		t.Fatalf("expected error about missing calendarId, got: %v", err)
	}
}

func TestCalendarEventsCmd_RejectsCalendarIdWithAll(t *testing.T) {
	flags := &rootFlags{Account: "test@example.com"}
	cmd := newCalendarEventsCmd(flags)
	cmd.SetArgs([]string{"--all", "primary"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "calendarId not allowed with --all flag") {
		t.Fatalf("expected error about calendarId with --all, got: %v", err)
	}
}


func TestEventWithCalendar_Sorting(t *testing.T) {
	// Test that events from different calendars are properly sorted
	events := []*eventWithCalendar{
		{
			Event: &calendar.Event{
				Id:      "e2",
				Start:   &calendar.EventDateTime{DateTime: "2025-01-15T14:00:00Z"},
				Summary: "Later Event",
			},
			CalendarID: "work@example.com",
		},
		{
			Event: &calendar.Event{
				Id:      "e1",
				Start:   &calendar.EventDateTime{DateTime: "2025-01-15T10:00:00Z"},
				Summary: "Earlier Event",
			},
			CalendarID: "primary",
		},
		{
			Event: &calendar.Event{
				Id:      "e3",
				Start:   &calendar.EventDateTime{DateTime: "2025-01-15T12:00:00Z"},
				Summary: "Middle Event",
			},
			CalendarID: "personal@example.com",
		},
	}

	sortEventsByStartTime(events)

	// Verify sorted order
	expected := []string{"e1", "e3", "e2"}
	for i, want := range expected {
		if events[i].Id != want {
			t.Errorf("events[%d].Id = %s, want %s", i, events[i].Id, want)
		}
	}
}

func TestEventWithCalendar_AllDayEvents(t *testing.T) {
	// Test sorting with all-day events
	events := []*eventWithCalendar{
		{
			Event: &calendar.Event{
				Id:      "e2",
				Start:   &calendar.EventDateTime{Date: "2025-01-16"},
				Summary: "Later All-Day",
			},
			CalendarID: "primary",
		},
		{
			Event: &calendar.Event{
				Id:      "e1",
				Start:   &calendar.EventDateTime{Date: "2025-01-15"},
				Summary: "Earlier All-Day",
			},
			CalendarID: "primary",
		},
	}

	sortEventsByStartTime(events)

	if events[0].Id != "e1" || events[1].Id != "e2" {
		t.Errorf("all-day events not sorted correctly")
	}
}

func TestEventWithCalendar_MixedEvents(t *testing.T) {
	// Test sorting with mixed all-day and timed events
	events := []*eventWithCalendar{
		{
			Event: &calendar.Event{
				Id:      "e2",
				Start:   &calendar.EventDateTime{DateTime: "2025-01-15T10:00:00Z"},
				Summary: "Timed Event",
			},
			CalendarID: "primary",
		},
		{
			Event: &calendar.Event{
				Id:      "e1",
				Start:   &calendar.EventDateTime{Date: "2025-01-15"},
				Summary: "All-Day Event",
			},
			CalendarID: "primary",
		},
	}

	sortEventsByStartTime(events)

	// All-day date "2025-01-15" should sort before timed event "2025-01-15T10:00:00Z"
	if events[0].Id != "e1" {
		t.Errorf("mixed events not sorted correctly: first event is %s, expected e1", events[0].Id)
	}
}
