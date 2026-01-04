package cmd

import "testing"

func TestBuildWorkingLocationProperties(t *testing.T) {
	cmd := &CalendarWorkingLocationCmd{Type: "home"}
	props, err := cmd.buildWorkingLocationProperties()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if props.Type != "homeOffice" || props.HomeOffice == nil {
		t.Fatalf("unexpected home props: %#v", props)
	}

	cmd = &CalendarWorkingLocationCmd{
		Type:        "office",
		OfficeLabel: "HQ",
		BuildingId:  "B1",
		FloorId:     "2",
		DeskId:      "D4",
	}
	props, err = cmd.buildWorkingLocationProperties()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if props.Type != "officeLocation" || props.OfficeLocation == nil {
		t.Fatalf("unexpected office props: %#v", props)
	}
	if props.OfficeLocation.Label != "HQ" || props.OfficeLocation.BuildingId != "B1" || props.OfficeLocation.FloorId != "2" || props.OfficeLocation.DeskId != "D4" {
		t.Fatalf("unexpected office details: %#v", props.OfficeLocation)
	}

	cmd = &CalendarWorkingLocationCmd{Type: "custom"}
	if _, buildErr := cmd.buildWorkingLocationProperties(); buildErr == nil {
		t.Fatalf("expected error for missing custom label")
	}

	cmd = &CalendarWorkingLocationCmd{Type: "custom", CustomLabel: "Cafe"}
	props, err = cmd.buildWorkingLocationProperties()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if props.Type != "customLocation" || props.CustomLocation == nil || props.CustomLocation.Label != "Cafe" {
		t.Fatalf("unexpected custom props: %#v", props)
	}

	cmd = &CalendarWorkingLocationCmd{Type: "invalid"}
	if _, buildErr := cmd.buildWorkingLocationProperties(); buildErr == nil {
		t.Fatalf("expected error for invalid type")
	}
}

func TestGenerateWorkingLocationSummary(t *testing.T) {
	cmd := &CalendarWorkingLocationCmd{Type: "home"}
	if got := cmd.generateSummary(); got != "Working from home" {
		t.Fatalf("unexpected summary: %q", got)
	}

	cmd = &CalendarWorkingLocationCmd{Type: "office", OfficeLabel: "HQ"}
	if got := cmd.generateSummary(); got != "Working from HQ" {
		t.Fatalf("unexpected summary: %q", got)
	}

	cmd = &CalendarWorkingLocationCmd{Type: "office"}
	if got := cmd.generateSummary(); got != "Working from office" {
		t.Fatalf("unexpected summary: %q", got)
	}

	cmd = &CalendarWorkingLocationCmd{Type: "custom", CustomLabel: "Cafe"}
	if got := cmd.generateSummary(); got != "Working from Cafe" {
		t.Fatalf("unexpected summary: %q", got)
	}

	cmd = &CalendarWorkingLocationCmd{Type: "invalid"}
	if got := cmd.generateSummary(); got != "Working location" {
		t.Fatalf("unexpected summary: %q", got)
	}
}
