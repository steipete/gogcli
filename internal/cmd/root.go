package cmd

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/steipete/gogcli/internal/errfmt"
	"github.com/steipete/gogcli/internal/outfmt"
	"github.com/steipete/gogcli/internal/ui"
)

type rootFlags struct {
	Color   string
	Account string
	JSON    bool
	Plain   bool
	Force   bool
	NoInput bool
	Verbose bool
}

func Execute(args []string) error {
	flags := rootFlags{Color: envOr("GOG_COLOR", "auto")}
	envMode := outfmt.FromEnv()
	flags.JSON = envMode.JSON
	flags.Plain = envMode.Plain

	// Avoid dangerous prefix-matching for commands (future-proofing).
	cobra.EnablePrefixMatching = false

	if hasExactArg(args, "--version") {
		fmt.Fprintln(os.Stdout, VersionString())
		return nil
	}

	root := &cobra.Command{
		Use:           "gog",
		Short:         "Google CLI for Gmail/Calendar/Drive/Contacts/Tasks/Sheets/Docs/Slides/People",
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
	  gog gmail thread get <threadId>
	  gog gmail get <messageId> --format metadata
	  gog gmail attachment <messageId> <attachmentId> --out ./attachment.bin
	  gog gmail labels get INBOX --json

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

	  # Sheets
	  gog sheets get <spreadsheetId> 'Sheet1!A1:C10'

	  # Exports
	  gog sheets export <spreadsheetId> --format pdf
	  gog docs export <docId> --format docx
	  gog slides export <presentationId> --format pptx

	  # Parseable output
	  gog --json drive ls --max 5 | jq .
	`),
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			logLevel := slog.LevelWarn
			if flags.Verbose {
				logLevel = slog.LevelDebug
			}
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: logLevel,
			})))

			mode, err := outfmt.FromFlags(flags.JSON, flags.Plain)
			if err != nil {
				return err
			}
			cmd.SetContext(outfmt.WithMode(cmd.Context(), mode))

			u, err := ui.New(ui.Options{
				Stdout: os.Stdout,
				Stderr: os.Stderr,
				Color: func() string {
					if outfmt.IsJSON(cmd.Context()) || outfmt.IsPlain(cmd.Context()) {
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
	root.PersistentFlags().StringVar(&flags.Account, "account", "", "Account email for API commands (gmail/calendar/drive/docs/slides/contacts/tasks/people/sheets)")
	root.PersistentFlags().BoolVar(&flags.JSON, "json", flags.JSON, "Output JSON to stdout (best for scripting)")
	root.PersistentFlags().BoolVar(&flags.Plain, "plain", flags.Plain, "Output stable, parseable text to stdout (TSV; no colors)")
	root.PersistentFlags().BoolVar(&flags.Force, "force", false, "Skip confirmations for destructive commands")
	root.PersistentFlags().BoolVar(&flags.NoInput, "no-input", false, "Never prompt; fail instead (useful for CI)")
	root.PersistentFlags().BoolVar(&flags.Verbose, "verbose", false, "Enable verbose logging")

	root.AddCommand(newAuthCmd(&flags))
	root.AddCommand(newDriveCmd(&flags))
	root.AddCommand(newDocsCmd(&flags))
	root.AddCommand(newSlidesCmd(&flags))
	root.AddCommand(newCalendarCmd(&flags))
	root.AddCommand(newGmailCmd(&flags))
	root.AddCommand(newContactsCmd(&flags))
	root.AddCommand(newTasksCmd(&flags))
	root.AddCommand(newPeopleCmd(&flags))
	root.AddCommand(newSheetsCmd(&flags))
	root.AddCommand(newVersionCmd())

	root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		// pflag already includes helpful context ("unknown flag", "invalid argument", ...).
		return newUsageError(err)
	})
	root.AddCommand(newCompletionCmd())

	err := root.Execute()
	if err == nil {
		return nil
	}
	if errors.Is(err, pflag.ErrHelp) {
		return nil
	}

	if ExitCode(err) == 1 && isUsageError(err) {
		err = &ExitError{Code: 2, Err: err}
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

func hasExactArg(args []string, target string) bool {
	for _, a := range args {
		if a == target {
			return true
		}
	}
	return false
}

// newUsageError wraps errors in a way main() can map to exit code 2.
func newUsageError(err error) error {
	if err == nil {
		return nil
	}
	// Preserve pflag.ErrHelp (should not be treated as failure).
	if errors.Is(err, pflag.ErrHelp) {
		return err
	}
	return &ExitError{Code: 2, Err: err}
}

func isUsageError(err error) bool {
	var outErr *outfmt.ParseError
	if errors.As(err, &outErr) {
		return true
	}
	var uiErr *ui.ParseError
	if errors.As(err, &uiErr) {
		return true
	}
	msg := strings.TrimSpace(err.Error())
	switch {
	case strings.HasPrefix(msg, "accepts "),
		strings.HasPrefix(msg, "requires "),
		strings.HasPrefix(msg, "unknown command"),
		strings.HasPrefix(msg, "invalid argument"),
		strings.HasPrefix(msg, "unknown flag"),
		strings.HasPrefix(msg, "unknown shorthand flag"):
		return true
	default:
		return false
	}
}
