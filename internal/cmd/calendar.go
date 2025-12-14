package cmd

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/steipete/gogcli/internal/googleapi"
	"github.com/steipete/gogcli/internal/outfmt"
	"github.com/steipete/gogcli/internal/ui"
	"google.golang.org/api/calendar/v3"
)

var newCalendarService = googleapi.NewCalendar

func newCalendarCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "calendar",
		Short: "Google Calendar",
	}
	cmd.AddCommand(newCalendarCalendarsCmd(flags))
	cmd.AddCommand(newCalendarAclCmd(flags))
	cmd.AddCommand(newCalendarEventsCmd(flags))
	cmd.AddCommand(newCalendarEventCmd(flags))
	cmd.AddCommand(newCalendarCreateCmd(flags))
	cmd.AddCommand(newCalendarUpdateCmd(flags))
	cmd.AddCommand(newCalendarDeleteCmd(flags))
	cmd.AddCommand(newCalendarFreeBusyCmd(flags))
	cmd.AddCommand(newCalendarRespondCmd(flags))
	cmd.AddCommand(newCalendarSearchCmd(flags))
	cmd.AddCommand(newCalendarColorsCmd(flags))
	cmd.AddCommand(newCalendarTimeCmd(flags))
	cmd.AddCommand(newCalendarConflictsCmd(flags))
	return cmd
}

func newCalendarCalendarsCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "calendars",
		Short: "List calendars",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}

			svc, err := newCalendarService(cmd.Context(), account)
			if err != nil {
				return err
			}

			resp, err := svc.CalendarList.List().Do()
			if err != nil {
				return err
			}
			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"calendars": resp.Items})
			}
			if len(resp.Items) == 0 {
				u.Err().Println("No calendars")
				return nil
			}

			tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tNAME\tROLE")
			for _, c := range resp.Items {
				fmt.Fprintf(tw, "%s\t%s\t%s\n", c.Id, c.Summary, c.AccessRole)
			}
			_ = tw.Flush()
			return nil
		},
	}
}

func newCalendarAclCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "acl <calendarId>",
		Short: "List access control rules for a calendar",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			calendarID := args[0]

			svc, err := newCalendarService(cmd.Context(), account)
			if err != nil {
				return err
			}

			resp, err := svc.Acl.List(calendarID).Do()
			if err != nil {
				return err
			}
			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"rules": resp.Items})
			}
			if len(resp.Items) == 0 {
				u.Err().Println("No ACL rules")
				return nil
			}

			tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "SCOPE_TYPE\tSCOPE_VALUE\tROLE")
			for _, rule := range resp.Items {
				scopeType := ""
				scopeValue := ""
				if rule.Scope != nil {
					scopeType = rule.Scope.Type
					scopeValue = rule.Scope.Value
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\n", scopeType, scopeValue, rule.Role)
			}
			_ = tw.Flush()
			return nil
		},
	}
}

func newCalendarEventsCmd(flags *rootFlags) *cobra.Command {
	var from string
	var to string
	var max int64
	var page string
	var query string
	var all bool

	cmd := &cobra.Command{
		Use:   "events <calendarIds>",
		Short: "List events from one or more calendars",
		Long: `List events from calendars. Supports comma-separated calendar IDs for multi-calendar queries.

Examples:
  gog calendar events primary
  gog calendar events primary,work@company.com --from 2025-01-01 --to 2025-01-07
  gog calendar events --all --from 2025-01-01 --to 2025-01-07`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}

			now := time.Now().UTC()
			oneWeekLater := now.Add(7 * 24 * time.Hour)
			if strings.TrimSpace(from) == "" {
				from = now.Format(time.RFC3339)
			}
			if strings.TrimSpace(to) == "" {
				to = oneWeekLater.Format(time.RFC3339)
			}

			svc, err := newCalendarService(cmd.Context(), account)
			if err != nil {
				return err
			}

			// Determine which calendars to query
			var calendarIDs []string
			if all {
				// Fetch all calendars
				resp, err := svc.CalendarList.List().Do()
				if err != nil {
					return err
				}
				for _, cal := range resp.Items {
					calendarIDs = append(calendarIDs, cal.Id)
				}
			} else if len(args) > 0 {
				calendarIDs = splitCSV(args[0])
			} else {
				calendarIDs = []string{"primary"}
			}

			if len(calendarIDs) == 0 {
				return errors.New("no calendar IDs specified")
			}

			// Single calendar - use original logic with pagination
			if len(calendarIDs) == 1 {
				calendarID := calendarIDs[0]
				call := svc.Events.List(calendarID).
					TimeMin(from).
					TimeMax(to).
					MaxResults(max).
					PageToken(page).
					SingleEvents(true).
					OrderBy("startTime")
				if strings.TrimSpace(query) != "" {
					call = call.Q(query)
				}
				resp, err := call.Do()
				if err != nil {
					return err
				}
				if outfmt.IsJSON(cmd.Context()) {
					return outfmt.WriteJSON(os.Stdout, map[string]any{
						"events":        resp.Items,
						"nextPageToken": resp.NextPageToken,
					})
				}

				if len(resp.Items) == 0 {
					u.Err().Println("No events")
					return nil
				}

				tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
				fmt.Fprintln(tw, "ID\tSTART\tEND\tSUMMARY")
				for _, e := range resp.Items {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", e.Id, eventStart(e), eventEnd(e), e.Summary)
				}
				_ = tw.Flush()

				if resp.NextPageToken != "" {
					u.Err().Printf("# Next page: --page %s", resp.NextPageToken)
				}
				return nil
			}

			// Multi-calendar - fetch from all and merge
			type eventWithCal struct {
				CalendarID string           `json:"calendarId"`
				Event      *calendar.Event  `json:"event"`
			}

			var allEvents []eventWithCal
			seenIDs := make(map[string]bool) // Deduplicate shared calendar events

			for _, calID := range calendarIDs {
				call := svc.Events.List(calID).
					TimeMin(from).
					TimeMax(to).
					MaxResults(max).
					SingleEvents(true).
					OrderBy("startTime")
				if strings.TrimSpace(query) != "" {
					call = call.Q(query)
				}
				resp, err := call.Do()
				if err != nil {
					u.Err().Printf("Warning: failed to fetch from %s: %v", calID, err)
					continue
				}
				for _, e := range resp.Items {
					if seenIDs[e.Id] {
						continue // Skip duplicates from shared calendars
					}
					seenIDs[e.Id] = true
					allEvents = append(allEvents, eventWithCal{CalendarID: calID, Event: e})
				}
			}

			// Sort by start time
			sort.Slice(allEvents, func(i, j int) bool {
				startI, _ := parseEventTimes(allEvents[i].Event)
				startJ, _ := parseEventTimes(allEvents[j].Event)
				return startI.Before(startJ)
			})

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{
					"events": allEvents,
				})
			}

			if len(allEvents) == 0 {
				u.Err().Println("No events")
				return nil
			}

			tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "CALENDAR\tID\tSTART\tEND\tSUMMARY")
			for _, ec := range allEvents {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
					ec.CalendarID, ec.Event.Id, eventStart(ec.Event), eventEnd(ec.Event), ec.Event.Summary)
			}
			_ = tw.Flush()
			return nil
		},
	}

	cmd.Flags().StringVar(&from, "from", "", "Start time (RFC3339; default: now)")
	cmd.Flags().StringVar(&to, "to", "", "End time (RFC3339; default: +7d)")
	cmd.Flags().Int64Var(&max, "max", 10, "Max results per calendar")
	cmd.Flags().StringVar(&page, "page", "", "Page token (single calendar only)")
	cmd.Flags().StringVar(&query, "query", "", "Free text search")
	cmd.Flags().BoolVar(&all, "all", false, "Query all calendars")
	return cmd
}

func newCalendarEventCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "event <calendarId> <eventId>",
		Short: "Get event details",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			calendarID := args[0]
			eventID := args[1]

			svc, err := newCalendarService(cmd.Context(), account)
			if err != nil {
				return err
			}

			e, err := svc.Events.Get(calendarID, eventID).Do()
			if err != nil {
				return err
			}
			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"event": e})
			}

			u.Out().Printf("id\t%s", e.Id)
			u.Out().Printf("summary\t%s", orEmpty(e.Summary, "(no title)"))
			u.Out().Printf("start\t%s", eventStart(e))
			u.Out().Printf("end\t%s", eventEnd(e))
			if e.Location != "" {
				u.Out().Printf("location\t%s", e.Location)
			}
			if e.Description != "" {
				u.Out().Printf("description\t%s", e.Description)
			}
			if len(e.Attendees) > 0 {
				addrs := make([]string, 0, len(e.Attendees))
				for _, a := range e.Attendees {
					if a != nil && a.Email != "" {
						addrs = append(addrs, a.Email)
					}
				}
				if len(addrs) > 0 {
					u.Out().Printf("attendees\t%s", strings.Join(addrs, ", "))
				}
			}
			if e.Status != "" {
				u.Out().Printf("status\t%s", e.Status)
			}
			if e.HtmlLink != "" {
				u.Out().Printf("link\t%s", e.HtmlLink)
			}
			return nil
		},
	}
}

func newCalendarCreateCmd(flags *rootFlags) *cobra.Command {
	var summary string
	var start string
	var end string
	var description string
	var location string
	var attendees string
	var allDay bool
	var organizer string
	var colorID string

	cmd := &cobra.Command{
		Use:   "create <calendarId>",
		Short: "Create a new event",
		Long: `Create a new calendar event.

Examples:
  gog calendar create primary --summary "Meeting" --start 2025-01-15T10:00:00Z --end 2025-01-15T11:00:00Z
  gog calendar create primary --summary "Vacation" --start 2025-01-20 --end 2025-01-25 --all-day
  gog calendar create primary --summary "Team Sync" --start ... --end ... --organizer team@company.com`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			calendarID := args[0]

			if strings.TrimSpace(summary) == "" || strings.TrimSpace(start) == "" || strings.TrimSpace(end) == "" {
				return errors.New("required: --summary, --start, --end")
			}

			svc, err := newCalendarService(cmd.Context(), account)
			if err != nil {
				return err
			}

			event := &calendar.Event{
				Summary:     summary,
				Description: description,
				Location:    location,
				Start:       buildEventDateTime(start, allDay),
				End:         buildEventDateTime(end, allDay),
				Attendees:   buildAttendees(attendees),
			}

			// Set organizer if specified (requires appropriate permissions/delegation)
			if organizer != "" {
				event.Organizer = &calendar.EventOrganizer{
					Email: organizer,
				}
			}

			// Set color if specified
			if colorID != "" {
				event.ColorId = colorID
			}

			created, err := svc.Events.Insert(calendarID, event).Do()
			if err != nil {
				return err
			}
			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"event": created})
			}
			u.Out().Printf("id\t%s", created.Id)
			if created.HtmlLink != "" {
				u.Out().Printf("link\t%s", created.HtmlLink)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&summary, "summary", "", "Event title (required)")
	cmd.Flags().StringVar(&start, "start", "", "Start time/date (required)")
	cmd.Flags().StringVar(&end, "end", "", "End time/date (required)")
	cmd.Flags().StringVar(&description, "description", "", "Event description")
	cmd.Flags().StringVar(&location, "location", "", "Event location")
	cmd.Flags().StringVar(&attendees, "attendees", "", "Attendees (comma-separated)")
	cmd.Flags().BoolVar(&allDay, "all-day", false, "Create all-day event (use YYYY-MM-DD for start/end)")
	cmd.Flags().StringVar(&organizer, "organizer", "", "Organizer email (requires delegation permissions)")
	cmd.Flags().StringVar(&colorID, "color", "", "Event color ID (use 'gog calendar colors' to list)")
	return cmd
}

func newCalendarUpdateCmd(flags *rootFlags) *cobra.Command {
	var summary string
	var start string
	var end string
	var description string
	var location string
	var attendees string
	var allDay bool

	cmd := &cobra.Command{
		Use:   "update <calendarId> <eventId>",
		Short: "Update an existing event",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			calendarID := args[0]
			eventID := args[1]

			svc, err := newCalendarService(cmd.Context(), account)
			if err != nil {
				return err
			}

			existing, err := svc.Events.Get(calendarID, eventID).Do()
			if err != nil {
				return err
			}

			targetAllDay := isAllDayEvent(existing)
			if cmd.Flags().Changed("all-day") {
				targetAllDay = allDay
				// Converting between all-day and timed needs explicit start/end.
				if !cmd.Flags().Changed("start") || !cmd.Flags().Changed("end") {
					return errors.New("when changing --all-day, also provide --start and --end")
				}
			}

			changed := false

			if cmd.Flags().Changed("summary") {
				existing.Summary = summary
				changed = true
			}
			if cmd.Flags().Changed("description") {
				existing.Description = description
				changed = true
			}
			if cmd.Flags().Changed("location") {
				existing.Location = location
				changed = true
			}

			if cmd.Flags().Changed("start") {
				existing.Start = buildEventDateTime(start, targetAllDay)
				changed = true
			}
			if cmd.Flags().Changed("end") {
				existing.End = buildEventDateTime(end, targetAllDay)
				changed = true
			}

			if cmd.Flags().Changed("attendees") {
				existing.Attendees = buildAttendees(attendees)
				changed = true
			}

			if !changed {
				return errors.New("no updates provided")
			}

			updated, err := svc.Events.Update(calendarID, eventID, existing).Do()
			if err != nil {
				return err
			}
			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"event": updated})
			}
			u.Out().Printf("id\t%s", updated.Id)
			if updated.HtmlLink != "" {
				u.Out().Printf("link\t%s", updated.HtmlLink)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&summary, "summary", "", "Event title")
	cmd.Flags().StringVar(&start, "start", "", "Start time/date (RFC3339 or YYYY-MM-DD)")
	cmd.Flags().StringVar(&end, "end", "", "End time/date (RFC3339 or YYYY-MM-DD)")
	cmd.Flags().StringVar(&description, "description", "", "Event description")
	cmd.Flags().StringVar(&location, "location", "", "Event location")
	cmd.Flags().StringVar(&attendees, "attendees", "", "Attendees (comma-separated)")
	cmd.Flags().BoolVar(&allDay, "all-day", false, "Treat start/end as all-day (YYYY-MM-DD)")
	return cmd
}

func newCalendarDeleteCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <calendarId> <eventId>",
		Short: "Delete an event",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			calendarID := args[0]
			eventID := args[1]

			svc, err := newCalendarService(cmd.Context(), account)
			if err != nil {
				return err
			}

			if err := svc.Events.Delete(calendarID, eventID).Do(); err != nil {
				return err
			}
			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{
					"deleted":    true,
					"calendarId": calendarID,
					"eventId":    eventID,
				})
			}
			u.Out().Printf("deleted\ttrue")
			u.Out().Printf("calendar_id\t%s", calendarID)
			u.Out().Printf("event_id\t%s", eventID)
			return nil
		},
	}
}

func newCalendarFreeBusyCmd(flags *rootFlags) *cobra.Command {
	var from string
	var to string

	cmd := &cobra.Command{
		Use:   "freebusy <calendarIds>",
		Short: "Check free/busy status for calendars (comma-separated IDs)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			calendarIDs := splitCSV(args[0])
			if len(calendarIDs) == 0 {
				return errors.New("no calendar IDs provided")
			}
			if strings.TrimSpace(from) == "" || strings.TrimSpace(to) == "" {
				return errors.New("required: --from and --to")
			}

			svc, err := newCalendarService(cmd.Context(), account)
			if err != nil {
				return err
			}

			items := make([]*calendar.FreeBusyRequestItem, 0, len(calendarIDs))
			for _, id := range calendarIDs {
				items = append(items, &calendar.FreeBusyRequestItem{Id: id})
			}

			resp, err := svc.Freebusy.Query(&calendar.FreeBusyRequest{
				TimeMin: from,
				TimeMax: to,
				Items:   items,
			}).Do()
			if err != nil {
				return err
			}
			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"calendars": resp.Calendars})
			}
			if len(resp.Calendars) == 0 {
				u.Err().Println("No data")
				return nil
			}

			tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "CALENDAR\tSTART\tEND")
			for id, data := range resp.Calendars {
				for _, b := range data.Busy {
					fmt.Fprintf(tw, "%s\t%s\t%s\n", id, b.Start, b.End)
				}
			}
			_ = tw.Flush()
			return nil
		},
	}

	cmd.Flags().StringVar(&from, "from", "", "Start time (RFC3339, required)")
	cmd.Flags().StringVar(&to, "to", "", "End time (RFC3339, required)")
	return cmd
}

func buildEventDateTime(value string, allDay bool) *calendar.EventDateTime {
	value = strings.TrimSpace(value)
	if allDay {
		return &calendar.EventDateTime{Date: value}
	}
	return &calendar.EventDateTime{DateTime: value}
}

func buildAttendees(csv string) []*calendar.EventAttendee {
	addrs := splitCSV(csv)
	if len(addrs) == 0 {
		return nil
	}
	out := make([]*calendar.EventAttendee, 0, len(addrs))
	for _, a := range addrs {
		out = append(out, &calendar.EventAttendee{Email: a})
	}
	return out
}

func splitCSV(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func eventStart(e *calendar.Event) string {
	if e == nil || e.Start == nil {
		return ""
	}
	if e.Start.DateTime != "" {
		return e.Start.DateTime
	}
	return e.Start.Date
}

func eventEnd(e *calendar.Event) string {
	if e == nil || e.End == nil {
		return ""
	}
	if e.End.DateTime != "" {
		return e.End.DateTime
	}
	return e.End.Date
}

func isAllDayEvent(e *calendar.Event) bool {
	return e != nil && e.Start != nil && e.Start.Date != ""
}

func orEmpty(s string, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

// newCalendarRespondCmd creates the respond command for RSVP to event invitations.
func newCalendarRespondCmd(flags *rootFlags) *cobra.Command {
	var status string
	var comment string

	cmd := &cobra.Command{
		Use:   "respond <calendarId> <eventId>",
		Short: "Respond to an event invitation (RSVP)",
		Long: `Respond to an event invitation with accept, decline, tentative, or needsAction.

Examples:
  gog calendar respond primary abc123 --status accept
  gog calendar respond primary abc123 --status decline --comment "Scheduling conflict"
  gog calendar respond work@company.com eventXYZ --status tentative`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			calendarID := args[0]
			eventID := args[1]

			// Validate status
			validStatuses := map[string]bool{
				"accepted":    true,
				"declined":    true,
				"tentative":   true,
				"needsAction": true,
				// Allow shorthand
				"accept":  true,
				"decline": true,
				"maybe":   true,
			}
			status = strings.TrimSpace(status)
			if status == "" {
				return errors.New("required: --status (accept, decline, tentative, needsAction)")
			}
			if !validStatuses[status] {
				return fmt.Errorf("invalid status %q: use accept, decline, tentative, or needsAction", status)
			}

			// Normalize shorthand to API values
			switch status {
			case "accept":
				status = "accepted"
			case "decline":
				status = "declined"
			case "maybe":
				status = "tentative"
			}

			svc, err := newCalendarService(cmd.Context(), account)
			if err != nil {
				return err
			}

			// Fetch the event first to get attendee list
			event, err := svc.Events.Get(calendarID, eventID).Do()
			if err != nil {
				return err
			}

			// Find and update the current user's attendee entry
			found := false
			for _, attendee := range event.Attendees {
				if attendee.Self {
					attendee.ResponseStatus = status
					if comment != "" {
						attendee.Comment = comment
					}
					found = true
					break
				}
			}

			if !found {
				return errors.New("you are not listed as an attendee on this event")
			}

			// Patch the event with updated attendee response
			updated, err := svc.Events.Patch(calendarID, eventID, &calendar.Event{
				Attendees: event.Attendees,
			}).Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{
					"responded":  true,
					"status":     status,
					"calendarId": calendarID,
					"eventId":    eventID,
				})
			}
			u.Out().Printf("responded\ttrue")
			u.Out().Printf("status\t%s", status)
			u.Out().Printf("event_id\t%s", updated.Id)
			return nil
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Response status: accept, decline, tentative, needsAction (required)")
	cmd.Flags().StringVar(&comment, "comment", "", "Optional comment with your response")
	return cmd
}

// newCalendarSearchCmd creates a dedicated search command for finding events.
func newCalendarSearchCmd(flags *rootFlags) *cobra.Command {
	var from string
	var to string
	var max int64
	var calendarID string

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search for events by text",
		Long: `Search for events matching a text query across event summary, description, location, and attendees.

Examples:
  gog calendar search "team meeting"
  gog calendar search "standup" --from 2025-01-01 --to 2025-03-01
  gog calendar search "project review" --calendar work@company.com --max 50`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			query := args[0]

			if calendarID == "" {
				calendarID = "primary"
			}

			// Default time range: past 30 days to future 90 days
			now := time.Now().UTC()
			if strings.TrimSpace(from) == "" {
				from = now.Add(-30 * 24 * time.Hour).Format(time.RFC3339)
			}
			if strings.TrimSpace(to) == "" {
				to = now.Add(90 * 24 * time.Hour).Format(time.RFC3339)
			}

			svc, err := newCalendarService(cmd.Context(), account)
			if err != nil {
				return err
			}

			resp, err := svc.Events.List(calendarID).
				Q(query).
				TimeMin(from).
				TimeMax(to).
				MaxResults(max).
				SingleEvents(true).
				OrderBy("startTime").
				Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{
					"query":  query,
					"events": resp.Items,
				})
			}

			if len(resp.Items) == 0 {
				u.Err().Printf("No events matching %q", query)
				return nil
			}

			tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tSTART\tEND\tSUMMARY")
			for _, e := range resp.Items {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", e.Id, eventStart(e), eventEnd(e), e.Summary)
			}
			_ = tw.Flush()
			return nil
		},
	}

	cmd.Flags().StringVar(&from, "from", "", "Start time (RFC3339; default: -30 days)")
	cmd.Flags().StringVar(&to, "to", "", "End time (RFC3339; default: +90 days)")
	cmd.Flags().Int64Var(&max, "max", 25, "Max results")
	cmd.Flags().StringVar(&calendarID, "calendar", "", "Calendar ID (default: primary)")
	return cmd
}

// newCalendarColorsCmd lists available calendar event colors.
func newCalendarColorsCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "colors",
		Short: "List available event and calendar colors",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}

			svc, err := newCalendarService(cmd.Context(), account)
			if err != nil {
				return err
			}

			colors, err := svc.Colors.Get().Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{
					"event":    colors.Event,
					"calendar": colors.Calendar,
				})
			}

			u.Out().Println("Event Colors:")
			tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tBACKGROUND\tFOREGROUND")
			for id, color := range colors.Event {
				fmt.Fprintf(tw, "%s\t%s\t%s\n", id, color.Background, color.Foreground)
			}
			_ = tw.Flush()

			u.Out().Println("\nCalendar Colors:")
			tw2 := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw2, "ID\tBACKGROUND\tFOREGROUND")
			for id, color := range colors.Calendar {
				fmt.Fprintf(tw2, "%s\t%s\t%s\n", id, color.Background, color.Foreground)
			}
			_ = tw2.Flush()
			return nil
		},
	}
}

// newCalendarTimeCmd shows current time in a calendar's timezone.
func newCalendarTimeCmd(flags *rootFlags) *cobra.Command {
	var timezone string

	cmd := &cobra.Command{
		Use:   "time [calendarId]",
		Short: "Show current time in calendar's timezone",
		Long: `Show the current date and time in a specific timezone.

Examples:
  gog calendar time                          # Uses primary calendar's timezone
  gog calendar time work@company.com         # Uses that calendar's timezone
  gog calendar time --timezone America/New_York`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}

			var tz *time.Location
			var tzName string

			if timezone != "" {
				// Use explicitly specified timezone
				tz, err = time.LoadLocation(timezone)
				if err != nil {
					return fmt.Errorf("invalid timezone %q: %w", timezone, err)
				}
				tzName = timezone
			} else {
				// Get timezone from calendar
				calendarID := "primary"
				if len(args) > 0 {
					calendarID = args[0]
				}

				svc, err := newCalendarService(cmd.Context(), account)
				if err != nil {
					return err
				}

				cal, err := svc.CalendarList.Get(calendarID).Do()
				if err != nil {
					return err
				}

				if cal.TimeZone != "" {
					tz, err = time.LoadLocation(cal.TimeZone)
					if err != nil {
						return fmt.Errorf("failed to load calendar timezone %q: %w", cal.TimeZone, err)
					}
					tzName = cal.TimeZone
				} else {
					tz = time.UTC
					tzName = "UTC"
				}
			}

			now := time.Now().In(tz)

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{
					"timezone":  tzName,
					"datetime":  now.Format(time.RFC3339),
					"date":      now.Format("2006-01-02"),
					"time":      now.Format("15:04:05"),
					"dayOfWeek": now.Weekday().String(),
				})
			}

			u.Out().Printf("timezone\t%s", tzName)
			u.Out().Printf("datetime\t%s", now.Format(time.RFC3339))
			u.Out().Printf("date\t%s", now.Format("2006-01-02"))
			u.Out().Printf("time\t%s", now.Format("15:04:05"))
			u.Out().Printf("day\t%s", now.Weekday().String())
			return nil
		},
	}

	cmd.Flags().StringVar(&timezone, "timezone", "", "Timezone (e.g., America/New_York)")
	return cmd
}

// newCalendarConflictsCmd detects overlapping events across calendars.
func newCalendarConflictsCmd(flags *rootFlags) *cobra.Command {
	var from string
	var to string

	cmd := &cobra.Command{
		Use:   "conflicts [calendarIds]",
		Short: "Detect overlapping events across calendars",
		Long: `Find events that overlap in time across one or more calendars.

Examples:
  gog calendar conflicts --from 2025-01-01 --to 2025-01-07
  gog calendar conflicts primary,work@company.com --from 2025-01-01 --to 2025-01-07`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}

			// Default time range
			now := time.Now().UTC()
			if strings.TrimSpace(from) == "" {
				from = now.Format(time.RFC3339)
			}
			if strings.TrimSpace(to) == "" {
				to = now.Add(7 * 24 * time.Hour).Format(time.RFC3339)
			}

			svc, err := newCalendarService(cmd.Context(), account)
			if err != nil {
				return err
			}

			// Get calendar IDs
			var calendarIDs []string
			if len(args) > 0 && args[0] != "" {
				calendarIDs = splitCSV(args[0])
			} else {
				// Fetch all calendars if none specified
				resp, err := svc.CalendarList.List().Do()
				if err != nil {
					return err
				}
				for _, cal := range resp.Items {
					calendarIDs = append(calendarIDs, cal.Id)
				}
			}

			if len(calendarIDs) == 0 {
				return errors.New("no calendars to check")
			}

			// Fetch all events from all calendars
			type eventWithCal struct {
				CalendarID string
				Event      *calendar.Event
				Start      time.Time
				End        time.Time
			}

			var allEvents []eventWithCal
			for _, calID := range calendarIDs {
				resp, err := svc.Events.List(calID).
					TimeMin(from).
					TimeMax(to).
					SingleEvents(true).
					OrderBy("startTime").
					Do()
				if err != nil {
					u.Err().Printf("Warning: failed to fetch events from %s: %v", calID, err)
					continue
				}
				for _, e := range resp.Items {
					start, end := parseEventTimes(e)
					if !start.IsZero() && !end.IsZero() {
						allEvents = append(allEvents, eventWithCal{
							CalendarID: calID,
							Event:      e,
							Start:      start,
							End:        end,
						})
					}
				}
			}

			// Find conflicts (O(n^2) but fine for reasonable event counts)
			type conflict struct {
				Event1Cal     string
				Event1ID      string
				Event1Summary string
				Event1Start   string
				Event1End     string
				Event2Cal     string
				Event2ID      string
				Event2Summary string
				Event2Start   string
				Event2End     string
			}

			var conflicts []conflict
			seen := make(map[string]bool) // Avoid duplicate pairs

			for i, e1 := range allEvents {
				for j, e2 := range allEvents {
					if i >= j {
						continue // Skip self and already-checked pairs
					}
					// Check for overlap: e1.Start < e2.End AND e2.Start < e1.End
					if e1.Start.Before(e2.End) && e2.Start.Before(e1.End) {
						// Skip if same event ID (shared calendar)
						if e1.Event.Id == e2.Event.Id {
							continue
						}
						key := fmt.Sprintf("%s:%s", e1.Event.Id, e2.Event.Id)
						if seen[key] {
							continue
						}
						seen[key] = true

						conflicts = append(conflicts, conflict{
							Event1Cal:     e1.CalendarID,
							Event1ID:      e1.Event.Id,
							Event1Summary: e1.Event.Summary,
							Event1Start:   eventStart(e1.Event),
							Event1End:     eventEnd(e1.Event),
							Event2Cal:     e2.CalendarID,
							Event2ID:      e2.Event.Id,
							Event2Summary: e2.Event.Summary,
							Event2Start:   eventStart(e2.Event),
							Event2End:     eventEnd(e2.Event),
						})
					}
				}
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{
					"conflicts": conflicts,
					"count":     len(conflicts),
				})
			}

			if len(conflicts) == 0 {
				u.Out().Println("No conflicts found")
				return nil
			}

			u.Out().Printf("Found %d conflict(s):\n", len(conflicts))
			for i, c := range conflicts {
				u.Out().Printf("\n--- Conflict %d ---", i+1)
				u.Out().Printf("Event 1: %s (%s)", c.Event1Summary, c.Event1Cal)
				u.Out().Printf("  Time: %s to %s", c.Event1Start, c.Event1End)
				u.Out().Printf("Event 2: %s (%s)", c.Event2Summary, c.Event2Cal)
				u.Out().Printf("  Time: %s to %s", c.Event2Start, c.Event2End)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&from, "from", "", "Start time (RFC3339; default: now)")
	cmd.Flags().StringVar(&to, "to", "", "End time (RFC3339; default: +7 days)")
	return cmd
}

// parseEventTimes extracts start and end times from an event.
func parseEventTimes(e *calendar.Event) (time.Time, time.Time) {
	var start, end time.Time
	if e == nil {
		return start, end
	}

	if e.Start != nil {
		if e.Start.DateTime != "" {
			start, _ = time.Parse(time.RFC3339, e.Start.DateTime)
		} else if e.Start.Date != "" {
			start, _ = time.Parse("2006-01-02", e.Start.Date)
		}
	}
	if e.End != nil {
		if e.End.DateTime != "" {
			end, _ = time.Parse(time.RFC3339, e.End.DateTime)
		} else if e.End.Date != "" {
			end, _ = time.Parse("2006-01-02", e.End.Date)
		}
	}
	return start, end
}

