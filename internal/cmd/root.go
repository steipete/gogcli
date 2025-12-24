package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steipete/gogcli/internal/errfmt"
	"github.com/steipete/gogcli/internal/outfmt"
	"github.com/steipete/gogcli/internal/ui"
)

type rootFlags struct {
	Color   string
	Account string
	Output  string
}

func Execute(args []string) error {
	flags := rootFlags{Color: envOr("GOG_COLOR", "auto")}
	flags.Output = envOr("GOG_OUTPUT", "text")

	root := &cobra.Command{
		Use:           "gog",
		Short:         "Google CLI for Gmail/Calendar/Drive/Contacts/Tasks/People",
		SilenceUsage:  true,
		SilenceErrors: true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		Example: strings.TrimSpace(`
  # One-time setup (OAuth)
  gog auth credentials ~/path/to/credentials.json
  gog auth add you@gmail.com

  # Avoid repeating --account
  export GOG_ACCOUNT=you@gmail.com

  # Gmail
  gog gmail search 'newer_than:7d' --max 10
  gog gmail thread <threadId>
  gog gmail get <messageId> --format metadata
  gog gmail attachment <messageId> <attachmentId> --out ./attachment.bin
  gog gmail labels get INBOX --output=json

  # Calendar
  gog calendar calendars
  gog calendar events <calendarId> --from 2025-01-01T00:00:00Z --to 2025-01-08T00:00:00Z --max 50
  gog calendar respond <calendarId> <eventId> --status accepted

  # Contacts
  gog contacts list --max 50
  gog contacts search "Ada" --max 50
  gog contacts other list --max 50

  # Tasks
  gog tasks lists --max 50
  gog tasks list <tasklistId> --max 50

  # People
  gog people me

  # Parseable output
  gog --output=json drive ls --max 5 | jq .
`),
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			mode, err := outfmt.Parse(flags.Output)
			if err != nil {
				return err
			}
			cmd.SetContext(outfmt.WithMode(cmd.Context(), mode))

			u, err := ui.New(ui.Options{
				Stdout: os.Stdout,
				Stderr: os.Stderr,
				Color: func() string {
					if outfmt.IsJSON(cmd.Context()) {
						return "never"
					}
					return flags.Color
				}(),
			})
			if err != nil {
				return err
			}
			cmd.SetContext(ui.WithUI(cmd.Context(), u))
			return nil
		},
	}

	root.SetArgs(args)
	root.PersistentFlags().StringVar(&flags.Color, "color", flags.Color, "Color output: auto|always|never")
	root.PersistentFlags().StringVar(&flags.Account, "account", "", "Account email for API commands (gmail/calendar/drive/contacts/tasks/people)")
	root.PersistentFlags().StringVar(&flags.Output, "output", flags.Output, "Output format: text|json")

	root.AddCommand(newAuthCmd())
	root.AddCommand(newDriveCmd(&flags))
	root.AddCommand(newCalendarCmd(&flags))
	root.AddCommand(newGmailCmd(&flags))
	root.AddCommand(newContactsCmd(&flags))
	root.AddCommand(newTasksCmd(&flags))
	root.AddCommand(newPeopleCmd(&flags))

	err := root.Execute()
	if err == nil {
		return nil
	}

	if u := ui.FromContext(root.Context()); u != nil {
		u.Err().Error(errfmt.Format(err))
		return err
	}
	_, _ = fmt.Fprintln(os.Stderr, errfmt.Format(err))
	return err
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
