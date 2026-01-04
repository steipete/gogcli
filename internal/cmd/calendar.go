package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"google.golang.org/api/calendar/v3"
	gapi "google.golang.org/api/googleapi"

	"github.com/steipete/gogcli/internal/googleapi"
	"github.com/steipete/gogcli/internal/outfmt"
	"github.com/steipete/gogcli/internal/ui"
)

var newCalendarService = googleapi.NewCalendar

const (
	scopeAll    = "all"
	scopeSingle = "single"
	scopeFuture = "future"
)

type CalendarCmd struct {
	Calendars       CalendarCalendarsCmd       `cmd:"" name:"calendars" help:"List calendars"`
	ACL             CalendarAclCmd             `cmd:"" name:"acl" help:"List calendar ACL"`
	Events          CalendarEventsCmd          `cmd:"" name:"events" help:"List events from a calendar or all calendars"`
	Event           CalendarEventCmd           `cmd:"" name:"event" help:"Get event"`
	Create          CalendarCreateCmd          `cmd:"" name:"create" help:"Create an event"`
	Update          CalendarUpdateCmd          `cmd:"" name:"update" help:"Update an event"`
	Delete          CalendarDeleteCmd          `cmd:"" name:"delete" help:"Delete an event"`
	FreeBusy        CalendarFreeBusyCmd        `cmd:"" name:"freebusy" help:"Get free/busy"`
	Respond         CalendarRespondCmd         `cmd:"" name:"respond" help:"Respond to an event invitation"`
	Colors          CalendarColorsCmd          `cmd:"" name:"colors" help:"Show calendar colors"`
	Conflicts       CalendarConflictsCmd       `cmd:"" name:"conflicts" help:"Find conflicts"`
	Search          CalendarSearchCmd          `cmd:"" name:"search" help:"Search events"`
	Time            CalendarTimeCmd            `cmd:"" name:"time" help:"Show server time"`
	FocusTime       CalendarFocusTimeCmd       `cmd:"" name:"focus-time" help:"Create a Focus Time block"`
	OOO             CalendarOOOCmd             `cmd:"" name:"out-of-office" aliases:"ooo" help:"Create an Out of Office event"`
	WorkingLocation CalendarWorkingLocationCmd `cmd:"" name:"working-location" aliases:"wl" help:"Set working location (home/office/custom)"`
}

type CalendarCalendarsCmd struct {
	Max  int64  `name:"max" aliases:"limit" help:"Max results" default:"100"`
	Page string `name:"page" help:"Page token"`
}

func (c *CalendarCalendarsCmd) Run(ctx context.Context, flags *RootFlags) error {
	u := ui.FromContext(ctx)
	account, err := requireAccount(flags)
	if err != nil {
		return err
	}

	svc, err := newCalendarService(ctx, account)
	if err != nil {
		return err
	}

	resp, err := svc.CalendarList.List().MaxResults(c.Max).PageToken(c.Page).Do()
	if err != nil {
		return err
	}
	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{
			"calendars":     resp.Items,
			"nextPageToken": resp.NextPageToken,
		})
	}
	if len(resp.Items) == 0 {
		u.Err().Println("No calendars")
		return nil
	}

	w, flush := tableWriter(ctx)
	defer flush()
	fmt.Fprintln(w, "ID\tNAME\tROLE")
	for _, cal := range resp.Items {
		fmt.Fprintf(w, "%s\t%s\t%s\n", cal.Id, cal.Summary, cal.AccessRole)
	}
	printNextPageHint(u, resp.NextPageToken)
	return nil
}

type CalendarAclCmd struct {
	CalendarID string `arg:"" name:"calendarId" help:"Calendar ID"`
	Max        int64  `name:"max" aliases:"limit" help:"Max results" default:"100"`
	Page       string `name:"page" help:"Page token"`
}

func (c *CalendarAclCmd) Run(ctx context.Context, flags *RootFlags) error {
	u := ui.FromContext(ctx)
	account, err := requireAccount(flags)
	if err != nil {
		return err
	}
	calendarID := strings.TrimSpace(c.CalendarID)
	if calendarID == "" {
		return usage("calendarId required")
	}

	svc, err := newCalendarService(ctx, account)
	if err != nil {
		return err
	}

	resp, err := svc.Acl.List(calendarID).MaxResults(c.Max).PageToken(c.Page).Do()
	if err != nil {
		return err
	}
	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{
			"rules":         resp.Items,
			"nextPageToken": resp.NextPageToken,
		})
	}
	if len(resp.Items) == 0 {
		u.Err().Println("No ACL rules")
		return nil
	}

	w, flush := tableWriter(ctx)
	defer flush()
	fmt.Fprintln(w, "SCOPE_TYPE\tSCOPE_VALUE\tROLE")
	for _, rule := range resp.Items {
		scopeType := ""
		scopeValue := ""
		if rule.Scope != nil {
			scopeType = rule.Scope.Type
			scopeValue = rule.Scope.Value
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", scopeType, scopeValue, rule.Role)
	}
	printNextPageHint(u, resp.NextPageToken)
	return nil
}

type CalendarEventsCmd struct {
	CalendarID        string `arg:"" name:"calendarId" optional:"" help:"Calendar ID"`
	From              string `name:"from" help:"Start time (RFC3339; default: now)"`
	To                string `name:"to" help:"End time (RFC3339; default: +7d)"`
	Max               int64  `name:"max" aliases:"limit" help:"Max results" default:"10"`
	Page              string `name:"page" help:"Page token"`
	Query             string `name:"query" help:"Free text search"`
	All               bool   `name:"all" help:"Fetch events from all calendars"`
	PrivatePropFilter string `name:"private-prop-filter" help:"Filter by private extended property (key=value)"`
	SharedPropFilter  string `name:"shared-prop-filter" help:"Filter by shared extended property (key=value)"`
	Fields            string `name:"fields" help:"Comma-separated fields to return"`
}

func (c *CalendarEventsCmd) Run(ctx context.Context, flags *RootFlags) error {
	account, err := requireAccount(flags)
	if err != nil {
		return err
	}

	if !c.All && strings.TrimSpace(c.CalendarID) == "" {
		return usage("calendarId required unless --all is specified")
	}
	if c.All && strings.TrimSpace(c.CalendarID) != "" {
		return usage("calendarId not allowed with --all flag")
	}

	now := time.Now().UTC()
	oneWeekLater := now.Add(7 * 24 * time.Hour)
	from := strings.TrimSpace(c.From)
	to := strings.TrimSpace(c.To)
	if from == "" {
		from = now.Format(time.RFC3339)
	}
	if to == "" {
		to = oneWeekLater.Format(time.RFC3339)
	}

	svc, err := newCalendarService(ctx, account)
	if err != nil {
		return err
	}

	if c.All {
		return listAllCalendarsEvents(ctx, svc, from, to, c.Max, c.Page, c.Query, c.PrivatePropFilter, c.SharedPropFilter, c.Fields)
	}
	calendarID := strings.TrimSpace(c.CalendarID)
	return listCalendarEvents(ctx, svc, calendarID, from, to, c.Max, c.Page, c.Query, c.PrivatePropFilter, c.SharedPropFilter, c.Fields)
}

type CalendarEventCmd struct {
	CalendarID string `arg:"" name:"calendarId" help:"Calendar ID"`
	EventID    string `arg:"" name:"eventId" help:"Event ID"`
}

func (c *CalendarEventCmd) Run(ctx context.Context, flags *RootFlags) error {
	u := ui.FromContext(ctx)
	account, err := requireAccount(flags)
	if err != nil {
		return err
	}
	calendarID := strings.TrimSpace(c.CalendarID)
	eventID := strings.TrimSpace(c.EventID)
	if calendarID == "" {
		return usage("empty calendarId")
	}
	if eventID == "" {
		return usage("empty eventId")
	}

	svc, err := newCalendarService(ctx, account)
	if err != nil {
		return err
	}

	event, err := svc.Events.Get(calendarID, eventID).Do()
	if err != nil {
		return err
	}
	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{"event": event})
	}
	printCalendarEvent(u, event)
	return nil
}

type CalendarCreateCmd struct {
	CalendarID            string   `arg:"" name:"calendarId" help:"Calendar ID"`
	Summary               string   `name:"summary" help:"Event summary/title"`
	From                  string   `name:"from" help:"Start time (RFC3339)"`
	To                    string   `name:"to" help:"End time (RFC3339)"`
	Description           string   `name:"description" help:"Description"`
	Location              string   `name:"location" help:"Location"`
	Attendees             string   `name:"attendees" help:"Comma-separated attendee emails"`
	AllDay                bool     `name:"all-day" help:"All-day event (use date-only in --from/--to)"`
	ColorId               string   `name:"event-color" help:"Event color ID (1-11). Use 'gog calendar colors' to see available colors."`
	Visibility            string   `name:"visibility" help:"Event visibility: default, public, private, confidential"`
	Transparency          string   `name:"transparency" help:"Show as busy (opaque) or free (transparent). Aliases: busy, free"`
	SendUpdates           string   `name:"send-updates" help:"Notification mode: all, externalOnly, none (default: all)"`
	GuestsCanInviteOthers *bool    `name:"guests-can-invite" help:"Allow guests to invite others"`
	GuestsCanModify       *bool    `name:"guests-can-modify" help:"Allow guests to modify event"`
	GuestsCanSeeOthers    *bool    `name:"guests-can-see-others" help:"Allow guests to see other guests"`
	WithMeet              bool     `name:"with-meet" help:"Create a Google Meet video conference for this event"`
	SourceUrl             string   `name:"source-url" help:"URL where event was created/imported from"`
	SourceTitle           string   `name:"source-title" help:"Title of the source"`
	Attachments           []string `name:"attachment" help:"File attachment URL (can be repeated)"`
	PrivateProps          []string `name:"private-prop" help:"Private extended property (key=value, can be repeated)"`
	SharedProps           []string `name:"shared-prop" help:"Shared extended property (key=value, can be repeated)"`
}

func (c *CalendarCreateCmd) Run(ctx context.Context, flags *RootFlags) error {
	u := ui.FromContext(ctx)
	account, err := requireAccount(flags)
	if err != nil {
		return err
	}
	calendarID := strings.TrimSpace(c.CalendarID)
	if calendarID == "" {
		return usage("empty calendarId")
	}

	if strings.TrimSpace(c.Summary) == "" || strings.TrimSpace(c.From) == "" || strings.TrimSpace(c.To) == "" {
		return usage("required: --summary, --from, --to")
	}

	colorId, err := validateColorId(c.ColorId)
	if err != nil {
		return err
	}
	visibility, err := validateVisibility(c.Visibility)
	if err != nil {
		return err
	}
	transparency, err := validateTransparency(c.Transparency)
	if err != nil {
		return err
	}
	sendUpdates, err := validateSendUpdates(c.SendUpdates)
	if err != nil {
		return err
	}

	svc, err := newCalendarService(ctx, account)
	if err != nil {
		return err
	}

	event := &calendar.Event{
		Summary:            strings.TrimSpace(c.Summary),
		Description:        strings.TrimSpace(c.Description),
		Location:           strings.TrimSpace(c.Location),
		Start:              buildEventDateTime(c.From, c.AllDay),
		End:                buildEventDateTime(c.To, c.AllDay),
		Attendees:          buildAttendees(c.Attendees),
		ColorId:            colorId,
		Visibility:         visibility,
		Transparency:       transparency,
		ConferenceData:     buildConferenceData(c.WithMeet),
		Attachments:        buildAttachments(c.Attachments),
		ExtendedProperties: buildExtendedProperties(c.PrivateProps, c.SharedProps),
	}
	if c.GuestsCanInviteOthers != nil {
		event.GuestsCanInviteOthers = c.GuestsCanInviteOthers
	}
	if c.GuestsCanModify != nil {
		event.GuestsCanModify = *c.GuestsCanModify
	}
	if c.GuestsCanSeeOthers != nil {
		event.GuestsCanSeeOtherGuests = c.GuestsCanSeeOthers
	}
	if strings.TrimSpace(c.SourceUrl) != "" {
		event.Source = &calendar.EventSource{
			Url:   strings.TrimSpace(c.SourceUrl),
			Title: strings.TrimSpace(c.SourceTitle),
		}
	}

	call := svc.Events.Insert(calendarID, event)
	if sendUpdates != "" {
		call = call.SendUpdates(sendUpdates)
	}
	if c.WithMeet {
		call = call.ConferenceDataVersion(1)
	}
	if len(event.Attachments) > 0 {
		call = call.SupportsAttachments(true)
	}
	created, err := call.Do()
	if err != nil {
		return err
	}
	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{"event": created})
	}
	printCalendarEvent(u, created)
	return nil
}

type CalendarUpdateCmd struct {
	CalendarID            string   `arg:"" name:"calendarId" help:"Calendar ID"`
	EventID               string   `arg:"" name:"eventId" help:"Event ID"`
	Summary               string   `name:"summary" help:"New summary/title (set empty to clear)"`
	From                  string   `name:"from" help:"New start time (RFC3339; set empty to clear)"`
	To                    string   `name:"to" help:"New end time (RFC3339; set empty to clear)"`
	Description           string   `name:"description" help:"New description (set empty to clear)"`
	Location              string   `name:"location" help:"New location (set empty to clear)"`
	Attendees             string   `name:"attendees" help:"Comma-separated attendee emails (set empty to clear)"`
	AllDay                bool     `name:"all-day" help:"All-day event (use date-only in --from/--to)"`
	ColorId               string   `name:"event-color" help:"Event color ID (1-11, or empty to clear)"`
	Visibility            string   `name:"visibility" help:"Event visibility: default, public, private, confidential"`
	Transparency          string   `name:"transparency" help:"Show as busy (opaque) or free (transparent). Aliases: busy, free"`
	GuestsCanInviteOthers *bool    `name:"guests-can-invite" help:"Allow guests to invite others"`
	GuestsCanModify       *bool    `name:"guests-can-modify" help:"Allow guests to modify event"`
	GuestsCanSeeOthers    *bool    `name:"guests-can-see-others" help:"Allow guests to see other guests"`
	Scope                 string   `name:"scope" help:"For recurring events: single, future, all" default:"all"`
	OriginalStartTime     string   `name:"original-start" help:"Original start time of instance (required for scope=single)"`
	PrivateProps          []string `name:"private-prop" help:"Private extended property (key=value, can be repeated)"`
	SharedProps           []string `name:"shared-prop" help:"Shared extended property (key=value, can be repeated)"`
}

func (c *CalendarUpdateCmd) Run(ctx context.Context, kctx *kong.Context, flags *RootFlags) error {
	u := ui.FromContext(ctx)
	account, err := requireAccount(flags)
	if err != nil {
		return err
	}
	calendarID := strings.TrimSpace(c.CalendarID)
	eventID := strings.TrimSpace(c.EventID)
	if calendarID == "" {
		return usage("empty calendarId")
	}
	if eventID == "" {
		return usage("empty eventId")
	}

	scope := strings.TrimSpace(strings.ToLower(c.Scope))
	if scope == "" {
		scope = scopeAll
	}
	switch scope {
	case scopeSingle:
		if strings.TrimSpace(c.OriginalStartTime) == "" {
			return usage("--original-start required when --scope=single")
		}
	case scopeFuture:
		if strings.TrimSpace(c.OriginalStartTime) == "" {
			return usage("--original-start required when --scope=future")
		}
	case scopeAll:
	default:
		return fmt.Errorf("invalid scope: %q (must be single, future, or all)", scope)
	}

	// If --all-day changed, require from/to to update both date/time fields.
	if flagProvided(kctx, "all-day") {
		if !flagProvided(kctx, "from") || !flagProvided(kctx, "to") {
			return usage("when changing --all-day, also provide --from and --to")
		}
	}

	patch := &calendar.Event{}
	changed := false
	if flagProvided(kctx, "summary") {
		patch.Summary = strings.TrimSpace(c.Summary)
		changed = true
	}
	if flagProvided(kctx, "description") {
		patch.Description = strings.TrimSpace(c.Description)
		changed = true
	}
	if flagProvided(kctx, "location") {
		patch.Location = strings.TrimSpace(c.Location)
		changed = true
	}
	if flagProvided(kctx, "from") {
		patch.Start = buildEventDateTime(c.From, c.AllDay)
		changed = true
	}
	if flagProvided(kctx, "to") {
		patch.End = buildEventDateTime(c.To, c.AllDay)
		changed = true
	}
	if flagProvided(kctx, "attendees") {
		patch.Attendees = buildAttendees(c.Attendees)
		changed = true
	}
	if flagProvided(kctx, "event-color") {
		colorId, colorErr := validateColorId(c.ColorId)
		if colorErr != nil {
			return colorErr
		}
		patch.ColorId = colorId
		changed = true
	}
	if flagProvided(kctx, "visibility") {
		visibility, visErr := validateVisibility(c.Visibility)
		if visErr != nil {
			return visErr
		}
		patch.Visibility = visibility
		changed = true
	}
	if flagProvided(kctx, "transparency") {
		transparency, transErr := validateTransparency(c.Transparency)
		if transErr != nil {
			return transErr
		}
		patch.Transparency = transparency
		changed = true
	}
	if flagProvided(kctx, "guests-can-invite") {
		if c.GuestsCanInviteOthers != nil {
			patch.GuestsCanInviteOthers = c.GuestsCanInviteOthers
		}
		patch.ForceSendFields = append(patch.ForceSendFields, "GuestsCanInviteOthers")
		changed = true
	}
	if flagProvided(kctx, "guests-can-modify") {
		if c.GuestsCanModify != nil {
			patch.GuestsCanModify = *c.GuestsCanModify
		}
		patch.ForceSendFields = append(patch.ForceSendFields, "GuestsCanModify")
		changed = true
	}
	if flagProvided(kctx, "guests-can-see-others") {
		if c.GuestsCanSeeOthers != nil {
			patch.GuestsCanSeeOtherGuests = c.GuestsCanSeeOthers
		}
		patch.ForceSendFields = append(patch.ForceSendFields, "GuestsCanSeeOtherGuests")
		changed = true
	}
	if flagProvided(kctx, "private-prop") || flagProvided(kctx, "shared-prop") {
		patch.ExtendedProperties = buildExtendedProperties(c.PrivateProps, c.SharedProps)
		changed = true
	}
	if !changed {
		return usage("no updates provided")
	}

	svc, err := newCalendarService(ctx, account)
	if err != nil {
		return err
	}

	targetEventID := eventID
	var parentRecurrence []string
	if scope == scopeFuture {
		parent, getErr := svc.Events.Get(calendarID, eventID).Context(ctx).Do()
		if getErr != nil {
			return getErr
		}
		if len(parent.Recurrence) == 0 {
			return fmt.Errorf("event %s is not a recurring event", eventID)
		}
		parentRecurrence = parent.Recurrence
		patch.Recurrence = parentRecurrence
	}
	if scope == scopeSingle || scope == scopeFuture {
		instanceID, resolveErr := resolveRecurringInstanceID(ctx, svc, calendarID, eventID, c.OriginalStartTime)
		if resolveErr != nil {
			return resolveErr
		}
		targetEventID = instanceID
	}

	updated, err := svc.Events.Patch(calendarID, targetEventID, patch).Do()
	if err != nil {
		return err
	}
	if scope == scopeFuture {
		truncated, truncateErr := truncateRecurrence(parentRecurrence, c.OriginalStartTime)
		if truncateErr != nil {
			return truncateErr
		}
		_, patchErr := svc.Events.Patch(calendarID, eventID, &calendar.Event{Recurrence: truncated}).Context(ctx).Do()
		if patchErr != nil {
			return patchErr
		}
	}
	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{"event": updated})
	}
	printCalendarEvent(u, updated)
	return nil
}

type CalendarDeleteCmd struct {
	CalendarID        string `arg:"" name:"calendarId" help:"Calendar ID"`
	EventID           string `arg:"" name:"eventId" help:"Event ID"`
	Scope             string `name:"scope" help:"For recurring events: single, future, all" default:"all"`
	OriginalStartTime string `name:"original-start" help:"Original start time of instance (required for scope=single)"`
}

func (c *CalendarDeleteCmd) Run(ctx context.Context, flags *RootFlags) error {
	u := ui.FromContext(ctx)
	account, err := requireAccount(flags)
	if err != nil {
		return err
	}
	calendarID := strings.TrimSpace(c.CalendarID)
	eventID := strings.TrimSpace(c.EventID)
	if calendarID == "" {
		return usage("empty calendarId")
	}
	if eventID == "" {
		return usage("empty eventId")
	}

	scope := strings.TrimSpace(strings.ToLower(c.Scope))
	if scope == "" {
		scope = scopeAll
	}
	switch scope {
	case scopeSingle:
		if strings.TrimSpace(c.OriginalStartTime) == "" {
			return usage("--original-start required when --scope=single")
		}
	case scopeFuture:
		if strings.TrimSpace(c.OriginalStartTime) == "" {
			return usage("--original-start required when --scope=future")
		}
	case scopeAll:
	default:
		return fmt.Errorf("invalid scope: %q (must be single, future, or all)", scope)
	}

	confirmMessage := fmt.Sprintf("delete event %s from calendar %s", eventID, calendarID)
	if scope == scopeSingle {
		confirmMessage = fmt.Sprintf("delete event %s (instance start %s) from calendar %s", eventID, c.OriginalStartTime, calendarID)
	}
	if scope == scopeFuture {
		confirmMessage = fmt.Sprintf("delete event %s (instance start %s) and all following from calendar %s", eventID, c.OriginalStartTime, calendarID)
	}
	if confirmErr := confirmDestructive(ctx, flags, confirmMessage); confirmErr != nil {
		return confirmErr
	}

	svc, err := newCalendarService(ctx, account)
	if err != nil {
		return err
	}

	targetEventID := eventID
	var parentRecurrence []string
	if scope == scopeFuture {
		parent, getErr := svc.Events.Get(calendarID, eventID).Context(ctx).Do()
		if getErr != nil {
			return getErr
		}
		if len(parent.Recurrence) == 0 {
			return fmt.Errorf("event %s is not a recurring event", eventID)
		}
		parentRecurrence = parent.Recurrence
	}
	if scope == scopeSingle || scope == scopeFuture {
		instanceID, resolveErr := resolveRecurringInstanceID(ctx, svc, calendarID, eventID, c.OriginalStartTime)
		if resolveErr != nil {
			return resolveErr
		}
		targetEventID = instanceID
	}

	if err := svc.Events.Delete(calendarID, targetEventID).Do(); err != nil {
		return err
	}
	if scope == scopeFuture {
		truncated, truncateErr := truncateRecurrence(parentRecurrence, c.OriginalStartTime)
		if truncateErr != nil {
			return truncateErr
		}
		_, patchErr := svc.Events.Patch(calendarID, eventID, &calendar.Event{Recurrence: truncated}).Context(ctx).Do()
		if patchErr != nil {
			return patchErr
		}
	}
	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{
			"deleted":    true,
			"calendarId": calendarID,
			"eventId":    targetEventID,
		})
	}
	u.Out().Printf("deleted\ttrue")
	u.Out().Printf("calendarId\t%s", calendarID)
	u.Out().Printf("eventId\t%s", targetEventID)
	return nil
}

type CalendarFreeBusyCmd struct {
	CalendarIDs string `arg:"" name:"calendarIds" help:"Comma-separated calendar IDs"`
	From        string `name:"from" help:"Start time (RFC3339, required)"`
	To          string `name:"to" help:"End time (RFC3339, required)"`
}

func (c *CalendarFreeBusyCmd) Run(ctx context.Context, flags *RootFlags) error {
	u := ui.FromContext(ctx)
	account, err := requireAccount(flags)
	if err != nil {
		return err
	}

	calendarIDs := splitCSV(c.CalendarIDs)
	if len(calendarIDs) == 0 {
		return usage("no calendar IDs provided")
	}
	if strings.TrimSpace(c.From) == "" || strings.TrimSpace(c.To) == "" {
		return usage("required: --from and --to")
	}

	svc, err := newCalendarService(ctx, account)
	if err != nil {
		return err
	}

	req := &calendar.FreeBusyRequest{
		TimeMin: c.From,
		TimeMax: c.To,
		Items:   make([]*calendar.FreeBusyRequestItem, 0, len(calendarIDs)),
	}
	for _, id := range calendarIDs {
		req.Items = append(req.Items, &calendar.FreeBusyRequestItem{Id: id})
	}

	resp, err := svc.Freebusy.Query(req).Do()
	if err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{"calendars": resp.Calendars})
	}

	if len(resp.Calendars) == 0 {
		u.Err().Println("No free/busy data")
		return nil
	}

	w, flush := tableWriter(ctx)
	defer flush()
	fmt.Fprintln(w, "CALENDAR\tSTART\tEND")
	for id, data := range resp.Calendars {
		for _, b := range data.Busy {
			fmt.Fprintf(w, "%s\t%s\t%s\n", id, b.Start, b.End)
		}
	}
	return nil
}

func listCalendarEvents(ctx context.Context, svc *calendar.Service, calendarID, from, to string, maxResults int64, page, query, privatePropFilter, sharedPropFilter, fields string) error {
	u := ui.FromContext(ctx)

	call := svc.Events.List(calendarID).
		TimeMin(from).
		TimeMax(to).
		MaxResults(maxResults).
		PageToken(page).
		SingleEvents(true).
		OrderBy("startTime")
	if strings.TrimSpace(query) != "" {
		call = call.Q(query)
	}
	if strings.TrimSpace(privatePropFilter) != "" {
		call = call.PrivateExtendedProperty(privatePropFilter)
	}
	if strings.TrimSpace(sharedPropFilter) != "" {
		call = call.SharedExtendedProperty(sharedPropFilter)
	}
	if strings.TrimSpace(fields) != "" {
		call = call.Fields(gapi.Field(fields))
	}
	resp, err := call.Context(ctx).Do()
	if err != nil {
		return err
	}
	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{
			"events":        resp.Items,
			"nextPageToken": resp.NextPageToken,
		})
	}

	if len(resp.Items) == 0 {
		u.Err().Println("No events")
		return nil
	}

	w, flush := tableWriter(ctx)
	defer flush()

	fmt.Fprintln(w, "ID\tSTART\tEND\tSUMMARY")
	for _, e := range resp.Items {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", e.Id, eventStart(e), eventEnd(e), e.Summary)
	}
	printNextPageHint(u, resp.NextPageToken)
	return nil
}

type eventWithCalendar struct {
	*calendar.Event
	CalendarID string
}

func listAllCalendarsEvents(ctx context.Context, svc *calendar.Service, from, to string, maxResults int64, page, query, privatePropFilter, sharedPropFilter, fields string) error {
	u := ui.FromContext(ctx)

	calResp, err := svc.CalendarList.List().Context(ctx).Do()
	if err != nil {
		return err
	}

	if len(calResp.Items) == 0 {
		u.Err().Println("No calendars")
		return nil
	}

	all := []*eventWithCalendar{}
	for _, cal := range calResp.Items {
		call := svc.Events.List(cal.Id).
			TimeMin(from).
			TimeMax(to).
			MaxResults(maxResults).
			PageToken(page).
			SingleEvents(true).
			OrderBy("startTime")
		if strings.TrimSpace(query) != "" {
			call = call.Q(query)
		}
		if strings.TrimSpace(privatePropFilter) != "" {
			call = call.PrivateExtendedProperty(privatePropFilter)
		}
		if strings.TrimSpace(sharedPropFilter) != "" {
			call = call.SharedExtendedProperty(sharedPropFilter)
		}
		if strings.TrimSpace(fields) != "" {
			call = call.Fields(gapi.Field(fields))
		}
		events, err := call.Context(ctx).Do()
		if err != nil {
			u.Err().Printf("calendar %s: %v", cal.Id, err)
			continue
		}
		for _, e := range events.Items {
			all = append(all, &eventWithCalendar{Event: e, CalendarID: cal.Id})
		}
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{"events": all})
	}
	if len(all) == 0 {
		u.Err().Println("No events")
		return nil
	}

	w, flush := tableWriter(ctx)
	defer flush()
	fmt.Fprintln(w, "CALENDAR\tID\tSTART\tEND\tSUMMARY")
	for _, e := range all {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", e.CalendarID, e.Id, eventStart(e.Event), eventEnd(e.Event), e.Summary)
	}
	return nil
}

func printCalendarEvent(u *ui.UI, event *calendar.Event) {
	if u == nil || event == nil {
		return
	}
	u.Out().Printf("id\t%s", event.Id)
	u.Out().Printf("summary\t%s", orEmpty(event.Summary, "(no title)"))
	if event.EventType != "" && event.EventType != "default" {
		u.Out().Printf("type\t%s", event.EventType)
	}
	u.Out().Printf("start\t%s", eventStart(event))
	u.Out().Printf("end\t%s", eventEnd(event))
	if event.Description != "" {
		u.Out().Printf("description\t%s", event.Description)
	}
	if event.Location != "" {
		u.Out().Printf("location\t%s", event.Location)
	}
	if event.ColorId != "" {
		u.Out().Printf("color\t%s", event.ColorId)
	}
	if event.Visibility != "" && event.Visibility != "default" {
		u.Out().Printf("visibility\t%s", event.Visibility)
	}
	if event.Transparency == "transparent" {
		u.Out().Printf("show-as\tfree")
	}
	if len(event.Attendees) > 0 {
		for _, a := range event.Attendees {
			if a == nil || strings.TrimSpace(a.Email) == "" {
				continue
			}
			status := a.ResponseStatus
			if a.Optional {
				status += " (optional)"
			}
			u.Out().Printf("attendee\t%s\t%s", strings.TrimSpace(a.Email), status)
		}
	}
	if event.GuestsCanInviteOthers != nil && !*event.GuestsCanInviteOthers {
		u.Out().Printf("guests-can-invite\tfalse")
	}
	if event.GuestsCanModify {
		u.Out().Printf("guests-can-modify\ttrue")
	}
	if event.GuestsCanSeeOtherGuests != nil && !*event.GuestsCanSeeOtherGuests {
		u.Out().Printf("guests-can-see-others\tfalse")
	}
	if event.HangoutLink != "" {
		u.Out().Printf("meet\t%s", event.HangoutLink)
	}
	if event.ConferenceData != nil && len(event.ConferenceData.EntryPoints) > 0 {
		for _, ep := range event.ConferenceData.EntryPoints {
			if ep.EntryPointType == "video" {
				u.Out().Printf("video-link\t%s", ep.Uri)
			}
		}
	}
	if len(event.Recurrence) > 0 {
		u.Out().Printf("recurrence\t%s", strings.Join(event.Recurrence, "; "))
	}
	if event.Reminders != nil {
		if event.Reminders.UseDefault {
			u.Out().Printf("reminders\t(calendar default)")
		} else if len(event.Reminders.Overrides) > 0 {
			reminders := make([]string, 0, len(event.Reminders.Overrides))
			for _, r := range event.Reminders.Overrides {
				if r != nil {
					reminders = append(reminders, fmt.Sprintf("%s:%dm", r.Method, r.Minutes))
				}
			}
			u.Out().Printf("reminders\t%s", strings.Join(reminders, ", "))
		}
	}
	if len(event.Attachments) > 0 {
		for _, a := range event.Attachments {
			if a != nil {
				u.Out().Printf("attachment\t%s", a.FileUrl)
			}
		}
	}
	if event.FocusTimeProperties != nil {
		u.Out().Printf("auto-decline\t%s", event.FocusTimeProperties.AutoDeclineMode)
		if event.FocusTimeProperties.ChatStatus != "" {
			u.Out().Printf("chat-status\t%s", event.FocusTimeProperties.ChatStatus)
		}
	}
	if event.OutOfOfficeProperties != nil {
		u.Out().Printf("auto-decline\t%s", event.OutOfOfficeProperties.AutoDeclineMode)
		if event.OutOfOfficeProperties.DeclineMessage != "" {
			u.Out().Printf("decline-message\t%s", event.OutOfOfficeProperties.DeclineMessage)
		}
	}
	if event.WorkingLocationProperties != nil {
		u.Out().Printf("location-type\t%s", event.WorkingLocationProperties.Type)
	}
	if event.Source != nil && event.Source.Url != "" {
		if event.Source.Title != "" {
			u.Out().Printf("source\t%s (%s)", event.Source.Url, event.Source.Title)
		} else {
			u.Out().Printf("source\t%s", event.Source.Url)
		}
	}
	if event.HtmlLink != "" {
		u.Out().Printf("link\t%s", event.HtmlLink)
	}
}

func buildEventDateTime(value string, allDay bool) *calendar.EventDateTime {
	value = strings.TrimSpace(value)
	if allDay {
		return &calendar.EventDateTime{Date: value}
	}
	return &calendar.EventDateTime{DateTime: value}
}

func buildConferenceData(withMeet bool) *calendar.ConferenceData {
	if !withMeet {
		return nil
	}
	return &calendar.ConferenceData{
		CreateRequest: &calendar.CreateConferenceRequest{
			RequestId: fmt.Sprintf("gogcli-%d", time.Now().UnixNano()),
			ConferenceSolutionKey: &calendar.ConferenceSolutionKey{
				Type: "hangoutsMeet",
			},
		},
	}
}

func buildRecurrence(rules []string) []string {
	if len(rules) == 0 {
		return nil
	}
	out := make([]string, 0, len(rules))
	for _, r := range rules {
		r = strings.TrimSpace(r)
		if r != "" {
			out = append(out, r)
		}
	}
	return out
}

func buildAttachments(urls []string) []*calendar.EventAttachment {
	if len(urls) == 0 {
		return nil
	}
	out := make([]*calendar.EventAttachment, 0, len(urls))
	for _, u := range urls {
		u = strings.TrimSpace(u)
		if u != "" {
			out = append(out, &calendar.EventAttachment{FileUrl: u})
		}
	}
	return out
}

func buildExtendedProperties(privateProps, sharedProps []string) *calendar.EventExtendedProperties {
	if len(privateProps) == 0 && len(sharedProps) == 0 {
		return nil
	}
	props := &calendar.EventExtendedProperties{}

	if len(privateProps) > 0 {
		props.Private = make(map[string]string)
		for _, p := range privateProps {
			if k, v, ok := strings.Cut(p, "="); ok {
				props.Private[strings.TrimSpace(k)] = strings.TrimSpace(v)
			}
		}
	}

	if len(sharedProps) > 0 {
		props.Shared = make(map[string]string)
		for _, p := range sharedProps {
			if k, v, ok := strings.Cut(p, "="); ok {
				props.Shared[strings.TrimSpace(k)] = strings.TrimSpace(v)
			}
		}
	}

	return props
}

func resolveRecurringInstanceID(ctx context.Context, svc *calendar.Service, calendarID, recurringEventID, originalStart string) (string, error) {
	originalStart = strings.TrimSpace(originalStart)
	if originalStart == "" {
		return "", fmt.Errorf("original start time required")
	}

	timeMin, timeMax, err := originalStartRange(originalStart)
	if err != nil {
		return "", err
	}

	call := svc.Events.Instances(calendarID, recurringEventID).
		ShowDeleted(false).
		TimeMin(timeMin).
		TimeMax(timeMax)

	for {
		resp, err := call.Context(ctx).Do()
		if err != nil {
			return "", err
		}
		for _, item := range resp.Items {
			if matchesOriginalStart(item, originalStart) {
				return item.Id, nil
			}
		}
		if resp.NextPageToken == "" {
			break
		}
		call = svc.Events.Instances(calendarID, recurringEventID).
			ShowDeleted(false).
			TimeMin(timeMin).
			TimeMax(timeMax).
			PageToken(resp.NextPageToken)
	}

	return "", fmt.Errorf("no instance found for original start %q", originalStart)
}

func matchesOriginalStart(event *calendar.Event, originalStart string) bool {
	if event == nil {
		return false
	}
	originalStart = strings.TrimSpace(originalStart)
	if event.OriginalStartTime != nil {
		if event.OriginalStartTime.DateTime == originalStart || event.OriginalStartTime.Date == originalStart {
			return true
		}
	}
	if event.Start != nil {
		if event.Start.DateTime == originalStart || event.Start.Date == originalStart {
			return true
		}
	}
	return false
}

func originalStartRange(originalStart string) (string, string, error) {
	if strings.Contains(originalStart, "T") {
		parsed, err := time.Parse(time.RFC3339, originalStart)
		if err != nil {
			parsed, err = time.Parse(time.RFC3339Nano, originalStart)
		}
		if err != nil {
			return "", "", fmt.Errorf("invalid original start time %q", originalStart)
		}
		return parsed.Format(time.RFC3339), parsed.Add(time.Minute).Format(time.RFC3339), nil
	}
	parsed, err := time.Parse("2006-01-02", originalStart)
	if err != nil {
		return "", "", fmt.Errorf("invalid original start date %q", originalStart)
	}
	return parsed.Format(time.RFC3339), parsed.Add(24 * time.Hour).Format(time.RFC3339), nil
}

func truncateRecurrence(rules []string, originalStart string) ([]string, error) {
	if len(rules) == 0 {
		return nil, fmt.Errorf("recurrence rules missing")
	}
	untilValue, err := recurrenceUntil(originalStart)
	if err != nil {
		return nil, err
	}

	updated := make([]string, 0, len(rules))
	foundRule := false
	for _, rule := range rules {
		trimmed := strings.TrimSpace(rule)
		upper := strings.ToUpper(trimmed)
		if !strings.HasPrefix(upper, "RRULE") {
			updated = append(updated, trimmed)
			continue
		}
		foundRule = true
		body := strings.TrimPrefix(trimmed, "RRULE:")
		if body == trimmed {
			body = strings.TrimPrefix(trimmed, "RRULE")
			body = strings.TrimPrefix(body, ":")
		}
		parts := strings.Split(body, ";")
		filtered := make([]string, 0, len(parts)+1)
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			upperPart := strings.ToUpper(part)
			if strings.HasPrefix(upperPart, "UNTIL=") || strings.HasPrefix(upperPart, "COUNT=") {
				continue
			}
			filtered = append(filtered, part)
		}
		filtered = append(filtered, "UNTIL="+untilValue)
		updated = append(updated, "RRULE:"+strings.Join(filtered, ";"))
	}
	if !foundRule {
		return nil, fmt.Errorf("recurrence has no RRULE")
	}
	return updated, nil
}

func recurrenceUntil(originalStart string) (string, error) {
	originalStart = strings.TrimSpace(originalStart)
	if originalStart == "" {
		return "", fmt.Errorf("original start time required")
	}
	if strings.Contains(originalStart, "T") {
		parsed, err := time.Parse(time.RFC3339, originalStart)
		if err != nil {
			parsed, err = time.Parse(time.RFC3339Nano, originalStart)
		}
		if err != nil {
			return "", fmt.Errorf("invalid original start time %q", originalStart)
		}
		until := parsed.Add(-time.Second).UTC()
		return until.Format("20060102T150405Z"), nil
	}
	parsed, err := time.Parse("2006-01-02", originalStart)
	if err != nil {
		return "", fmt.Errorf("invalid original start date %q", originalStart)
	}
	until := parsed.AddDate(0, 0, -1)
	return until.Format("20060102"), nil
}

func buildAttendees(csv string) []*calendar.EventAttendee {
	addrs := splitCSV(csv)
	if len(addrs) == 0 {
		return nil
	}
	out := make([]*calendar.EventAttendee, 0, len(addrs))
	for _, a := range addrs {
		attendee := parseAttendee(a)
		if attendee != nil {
			out = append(out, attendee)
		}
	}
	return out
}

func parseAttendee(s string) *calendar.EventAttendee {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ";")
	email := strings.TrimSpace(parts[0])
	if email == "" {
		return nil
	}

	attendee := &calendar.EventAttendee{Email: email}
	for _, p := range parts[1:] {
		raw := strings.TrimSpace(p)
		lower := strings.ToLower(raw)
		if lower == "optional" {
			attendee.Optional = true
			continue
		}
		if strings.HasPrefix(lower, "comment=") {
			attendee.Comment = strings.TrimSpace(raw[len("comment="):])
		}
	}
	return attendee
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

func validateColorId(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	id, err := strconv.Atoi(s)
	if err != nil {
		return "", fmt.Errorf("invalid color ID: %q (must be 1-11)", s)
	}
	if id < 1 || id > 11 {
		return "", fmt.Errorf("color ID must be 1-11 (got %d)", id)
	}
	return s, nil
}

func validateVisibility(s string) (string, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return "", nil
	}
	valid := map[string]bool{
		"default":      true,
		"public":       true,
		"private":      true,
		"confidential": true,
	}
	if !valid[s] {
		return "", fmt.Errorf("invalid visibility: %q (must be default, public, private, or confidential)", s)
	}
	return s, nil
}

func validateTransparency(s string) (string, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return "", nil
	}
	switch s {
	case "busy":
		return "opaque", nil
	case "free":
		return "transparent", nil
	case "opaque", "transparent":
		return s, nil
	default:
		return "", fmt.Errorf("invalid transparency: %q (must be opaque/busy or transparent/free)", s)
	}
}

func validateSendUpdates(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	switch strings.ToLower(s) {
	case "all":
		return "all", nil
	case "externalonly":
		return "externalOnly", nil
	case "none":
		return "none", nil
	default:
		return "", fmt.Errorf("invalid send-updates value: %q (must be all, externalOnly, or none)", s)
	}
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
