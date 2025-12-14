package cmd

import (
	"errors"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/steipete/gogcli/internal/outfmt"
	"github.com/steipete/gogcli/internal/ui"
	"google.golang.org/api/gmail/v1"
)

func newGmailVacationCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vacation",
		Short: "Manage vacation responder settings",
	}

	cmd.AddCommand(newGmailVacationGetCmd(flags))
	cmd.AddCommand(newGmailVacationUpdateCmd(flags))
	return cmd
}

func newGmailVacationGetCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "Get current vacation responder settings",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}

			svc, err := newGmailService(cmd.Context(), account)
			if err != nil {
				return err
			}

			vacation, err := svc.Users.Settings.GetVacation("me").Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"vacation": vacation})
			}

			u.Out().Printf("enable_auto_reply\t%t", vacation.EnableAutoReply)
			u.Out().Printf("response_subject\t%s", vacation.ResponseSubject)
			u.Out().Printf("response_body_html\t%s", vacation.ResponseBodyHtml)
			u.Out().Printf("response_body_plain_text\t%s", vacation.ResponseBodyPlainText)
			if vacation.StartTime != 0 {
				u.Out().Printf("start_time\t%d", vacation.StartTime)
			}
			if vacation.EndTime != 0 {
				u.Out().Printf("end_time\t%d", vacation.EndTime)
			}
			u.Out().Printf("restrict_to_contacts\t%t", vacation.RestrictToContacts)
			u.Out().Printf("restrict_to_domain\t%t", vacation.RestrictToDomain)
			return nil
		},
	}
}

func newGmailVacationUpdateCmd(flags *rootFlags) *cobra.Command {
	var enable bool
	var disable bool
	var subject string
	var body string
	var startTime string
	var endTime string
	var contactsOnly bool
	var domainOnly bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update vacation responder settings",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}

			if enable && disable {
				return errors.New("cannot specify both --enable and --disable")
			}

			svc, err := newGmailService(cmd.Context(), account)
			if err != nil {
				return err
			}

			// Get current settings first
			current, err := svc.Users.Settings.GetVacation("me").Do()
			if err != nil {
				return err
			}

			// Build update request, preserving existing values if not specified
			vacation := &gmail.VacationSettings{
				EnableAutoReply:       current.EnableAutoReply,
				ResponseSubject:       current.ResponseSubject,
				ResponseBodyHtml:      current.ResponseBodyHtml,
				ResponseBodyPlainText: current.ResponseBodyPlainText,
				StartTime:             current.StartTime,
				EndTime:               current.EndTime,
				RestrictToContacts:    current.RestrictToContacts,
				RestrictToDomain:      current.RestrictToDomain,
			}

			// Apply flags
			if enable {
				vacation.EnableAutoReply = true
			}
			if disable {
				vacation.EnableAutoReply = false
			}
			if cmd.Flags().Changed("subject") {
				vacation.ResponseSubject = subject
			}
			if cmd.Flags().Changed("body") {
				vacation.ResponseBodyHtml = body
				vacation.ResponseBodyPlainText = stripHTML(body)
			}
			if cmd.Flags().Changed("start") {
				t, err := parseRFC3339ToMillis(startTime)
				if err != nil {
					return err
				}
				vacation.StartTime = t
			}
			if cmd.Flags().Changed("end") {
				t, err := parseRFC3339ToMillis(endTime)
				if err != nil {
					return err
				}
				vacation.EndTime = t
			}
			if cmd.Flags().Changed("contacts-only") {
				vacation.RestrictToContacts = contactsOnly
			}
			if cmd.Flags().Changed("domain-only") {
				vacation.RestrictToDomain = domainOnly
			}

			updated, err := svc.Users.Settings.UpdateVacation("me", vacation).Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"vacation": updated})
			}

			u.Out().Println("Vacation responder updated successfully")
			u.Out().Printf("enable_auto_reply\t%t", updated.EnableAutoReply)
			u.Out().Printf("response_subject\t%s", updated.ResponseSubject)
			if updated.StartTime != 0 {
				u.Out().Printf("start_time\t%d", updated.StartTime)
			}
			if updated.EndTime != 0 {
				u.Out().Printf("end_time\t%d", updated.EndTime)
			}
			u.Out().Printf("restrict_to_contacts\t%t", updated.RestrictToContacts)
			u.Out().Printf("restrict_to_domain\t%t", updated.RestrictToDomain)
			return nil
		},
	}

	cmd.Flags().BoolVar(&enable, "enable", false, "Enable vacation responder")
	cmd.Flags().BoolVar(&disable, "disable", false, "Disable vacation responder")
	cmd.Flags().StringVar(&subject, "subject", "", "Subject line for auto-reply")
	cmd.Flags().StringVar(&body, "body", "", "HTML body of the auto-reply message")
	cmd.Flags().StringVar(&startTime, "start", "", "Start time in RFC3339 format (e.g., 2024-12-20T00:00:00Z)")
	cmd.Flags().StringVar(&endTime, "end", "", "End time in RFC3339 format (e.g., 2024-12-31T23:59:59Z)")
	cmd.Flags().BoolVar(&contactsOnly, "contacts-only", false, "Only respond to contacts")
	cmd.Flags().BoolVar(&domainOnly, "domain-only", false, "Only respond to same domain")
	return cmd
}

func parseRFC3339ToMillis(rfc3339 string) (int64, error) {
	if rfc3339 == "" {
		return 0, nil
	}
	// Parse RFC3339 format and convert to milliseconds since epoch
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		return 0, err
	}
	return t.UnixMilli(), nil
}

func stripHTML(html string) string {
	// Simple HTML stripping for plain text version
	// This is a basic implementation - Gmail API will handle more complex conversions
	// For now, just return the HTML as-is, Gmail will auto-convert
	return html
}
