package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steipete/gogcli/internal/outfmt"
	"github.com/steipete/gogcli/internal/ui"
)

func newCalendarRespondCmd(flags *rootFlags) *cobra.Command {
	var status string
	var comment string

	cmd := &cobra.Command{
		Use:   "respond <calendarId> <eventId>",
		Short: "Respond to a calendar event invitation",
		Long: `Respond to a calendar event invitation with accepted, declined, tentative, or needsAction.

Status values:
  - accepted: Accept the invitation
  - declined: Decline the invitation
  - tentative: Mark as tentative (maybe)
  - needsAction: Reset to needs action

You can optionally include a comment with your response.`,
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
			status = strings.TrimSpace(status)
			validStatuses := []string{"accepted", "declined", "tentative", "needsAction"}
			isValid := false
			for _, v := range validStatuses {
				if status == v {
					isValid = true
					break
				}
			}
			if !isValid {
				return fmt.Errorf("invalid status %q; must be one of: %s", status, strings.Join(validStatuses, ", "))
			}

			svc, err := newCalendarService(cmd.Context(), account)
			if err != nil {
				return err
			}

			// Get the event
			event, err := svc.Events.Get(calendarID, eventID).Do()
			if err != nil {
				return err
			}

			// Find the authenticated user in attendees
			if len(event.Attendees) == 0 {
				return errors.New("event has no attendees")
			}

			var selfAttendee *int
			for i, a := range event.Attendees {
				if a.Self {
					selfAttendee = &i
					break
				}
			}

			if selfAttendee == nil {
				return errors.New("you are not an attendee of this event")
			}

			// Check if user is the organizer
			if event.Attendees[*selfAttendee].Organizer {
				return errors.New("cannot respond to your own event (you are the organizer)")
			}

			// Update the attendee's response status
			event.Attendees[*selfAttendee].ResponseStatus = status
			if strings.TrimSpace(comment) != "" {
				event.Attendees[*selfAttendee].Comment = comment
			}

			// Patch the event with updated attendees
			updated, err := svc.Events.Patch(calendarID, eventID, event).Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"event": updated})
			}

			u.Out().Printf("id\t%s", updated.Id)
			u.Out().Printf("summary\t%s", orEmpty(updated.Summary, "(no title)"))
			u.Out().Printf("response_status\t%s", status)
			if strings.TrimSpace(comment) != "" {
				u.Out().Printf("comment\t%s", comment)
			}
			if updated.HtmlLink != "" {
				u.Out().Printf("link\t%s", updated.HtmlLink)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Response status (accepted, declined, tentative, needsAction) (required)")
	cmd.Flags().StringVar(&comment, "comment", "", "Optional comment/note to include with response")
	_ = cmd.MarkFlagRequired("status")

	return cmd
}
