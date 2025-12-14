package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/steipete/gogcli/internal/outfmt"
	"github.com/steipete/gogcli/internal/ui"
)

func newCalendarSearchCmd(flags *rootFlags) *cobra.Command {
	var from string
	var to string
	var calendarID string
	var max int64

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search for events by text query across calendars",
		Long: `Search for calendar events matching a text query.

The query searches across event titles, descriptions, locations, and attendees.
Default time range is 30 days ago to 90 days from now.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			query := strings.TrimSpace(args[0])
			if query == "" {
				return fmt.Errorf("search query cannot be empty")
			}

			// Calculate default time range if not specified
			now := time.Now().UTC()
			thirtyDaysAgo := now.Add(-30 * 24 * time.Hour)
			ninetyDaysLater := now.Add(90 * 24 * time.Hour)

			if strings.TrimSpace(from) == "" {
				from = thirtyDaysAgo.Format(time.RFC3339)
			}
			if strings.TrimSpace(to) == "" {
				to = ninetyDaysLater.Format(time.RFC3339)
			}

			svc, err := newCalendarService(cmd.Context(), account)
			if err != nil {
				return err
			}

			call := svc.Events.List(calendarID).
				Q(query).
				TimeMin(from).
				TimeMax(to).
				MaxResults(max).
				SingleEvents(true).
				OrderBy("startTime")

			resp, err := call.Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{
					"events": resp.Items,
					"query":  query,
				})
			}

			if len(resp.Items) == 0 {
				u.Err().Println("No events found")
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

	cmd.Flags().StringVar(&from, "from", "", "Start time (RFC3339; default: 30 days ago)")
	cmd.Flags().StringVar(&to, "to", "", "End time (RFC3339; default: 90 days from now)")
	cmd.Flags().StringVar(&calendarID, "calendar", "primary", "Calendar ID")
	cmd.Flags().Int64Var(&max, "max", 25, "Max results")

	return cmd
}
