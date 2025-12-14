package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/steipete/gogcli/internal/outfmt"
	"github.com/steipete/gogcli/internal/ui"
)

func newCalendarTimeCmd(flags *rootFlags) *cobra.Command {
	var calendarID string
	var timezone string

	cmd := &cobra.Command{
		Use:   "time",
		Short: "Show current time in a calendar's timezone",
		Long: `Show the current time in a calendar's timezone or a specified timezone.

If --timezone is provided, uses that timezone directly.
Otherwise, retrieves the timezone from the specified calendar (default: "primary").`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}

			var tz string
			var loc *time.Location

			if timezone != "" {
				// Use provided timezone
				tz = timezone
				loc, err = time.LoadLocation(timezone)
				if err != nil {
					return fmt.Errorf("invalid timezone %q: %w", timezone, err)
				}
			} else {
				// Get timezone from calendar
				svc, err := newCalendarService(cmd.Context(), account)
				if err != nil {
					return err
				}

				cal, err := svc.CalendarList.Get(calendarID).Do()
				if err != nil {
					return fmt.Errorf("failed to get calendar %q: %w", calendarID, err)
				}

				tz = cal.TimeZone
				if tz == "" {
					return fmt.Errorf("calendar %q has no timezone set", calendarID)
				}

				loc, err = time.LoadLocation(tz)
				if err != nil {
					return fmt.Errorf("invalid calendar timezone %q: %w", tz, err)
				}
			}

			// Get current time in the timezone
			now := time.Now().In(loc)
			formatted := now.Format("Monday, January 02, 2006 03:04 PM")

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{
					"timezone":     tz,
					"current_time": now.Format(time.RFC3339),
					"formatted":    formatted,
				})
			}

			// Table output
			u.Out().Printf("timezone\t%s", tz)
			u.Out().Printf("current_time\t%s", now.Format(time.RFC3339))
			u.Out().Printf("formatted\t%s", formatted)
			return nil
		},
	}

	cmd.Flags().StringVar(&calendarID, "calendar", "primary", "Calendar ID to get timezone from")
	cmd.Flags().StringVar(&timezone, "timezone", "", "Override timezone (e.g., America/New_York, UTC)")
	return cmd
}
