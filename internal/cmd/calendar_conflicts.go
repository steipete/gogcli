package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/steipete/gogcli/internal/outfmt"
	"github.com/steipete/gogcli/internal/ui"
	"google.golang.org/api/calendar/v3"
)

type conflict struct {
	Start     string   `json:"start"`
	End       string   `json:"end"`
	Calendars []string `json:"calendars"`
}

func newCalendarConflictsCmd(flags *rootFlags) *cobra.Command {
	var from string
	var to string
	var calendars string

	cmd := &cobra.Command{
		Use:   "conflicts",
		Short: "Detect overlapping/conflicting events across calendars",
		Long: `Detect overlapping busy periods across multiple calendars.

A conflict occurs when the same time slot has busy periods in 2+ calendars.
Uses the FreeBusy API to check calendar availability.

Default time range is now to +7 days.
Default calendars: "primary"`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}

			// Parse time range
			now := time.Now().UTC()
			sevenDaysLater := now.Add(7 * 24 * time.Hour)
			if strings.TrimSpace(from) == "" {
				from = now.Format(time.RFC3339)
			}
			if strings.TrimSpace(to) == "" {
				to = sevenDaysLater.Format(time.RFC3339)
			}

			// Parse calendar IDs
			calendarIDs := splitCSV(calendars)
			if len(calendarIDs) == 0 {
				return errors.New("no calendar IDs provided")
			}

			svc, err := newCalendarService(cmd.Context(), account)
			if err != nil {
				return err
			}

			// Build FreeBusy request
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

			// Detect conflicts
			conflicts := detectConflicts(resp.Calendars)

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{
					"conflicts": conflicts,
					"count":     len(conflicts),
				})
			}

			// Table output
			if len(conflicts) == 0 {
				u.Out().Println("No conflicts found")
				return nil
			}

			fmt.Printf("CONFLICTS FOUND: %d\n\n", len(conflicts))
			tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "START\tEND\tCALENDARS")
			for _, c := range conflicts {
				fmt.Fprintf(tw, "%s\t%s\t%s\n", c.Start, c.End, strings.Join(c.Calendars, ", "))
			}
			_ = tw.Flush()
			return nil
		},
	}

	cmd.Flags().StringVar(&from, "from", "", "Start time (RFC3339; default: now)")
	cmd.Flags().StringVar(&to, "to", "", "End time (RFC3339; default: +7d)")
	cmd.Flags().StringVar(&calendars, "calendars", "primary", "Comma-separated calendar IDs")
	return cmd
}

// detectConflicts finds overlapping busy periods across calendars
func detectConflicts(calendars map[string]calendar.FreeBusyCalendar) []conflict {
	if len(calendars) < 2 {
		// Need at least 2 calendars to have conflicts
		return []conflict{}
	}

	// Collect all busy periods with their calendar IDs
	type busyPeriod struct {
		start      time.Time
		end        time.Time
		calendarID string
	}

	var allBusy []busyPeriod
	for calID, cal := range calendars {
		for _, b := range cal.Busy {
			start, err := time.Parse(time.RFC3339, b.Start)
			if err != nil {
				continue
			}
			end, err := time.Parse(time.RFC3339, b.End)
			if err != nil {
				continue
			}
			allBusy = append(allBusy, busyPeriod{
				start:      start,
				end:        end,
				calendarID: calID,
			})
		}
	}

	// Find overlapping periods
	var conflicts []conflict
	seen := make(map[string]bool)

	for i := 0; i < len(allBusy); i++ {
		for j := i + 1; j < len(allBusy); j++ {
			a := allBusy[i]
			b := allBusy[j]

			// Skip if same calendar
			if a.calendarID == b.calendarID {
				continue
			}

			// Check if they overlap: a.start < b.end AND a.end > b.start
			if a.start.Before(b.end) && a.end.After(b.start) {
				// Calculate overlap period
				overlapStart := a.start
				if b.start.After(a.start) {
					overlapStart = b.start
				}
				overlapEnd := a.end
				if b.end.Before(a.end) {
					overlapEnd = b.end
				}

				// Create conflict key to avoid duplicates
				calendarsInvolved := []string{a.calendarID, b.calendarID}
				if a.calendarID > b.calendarID {
					calendarsInvolved = []string{b.calendarID, a.calendarID}
				}
				key := fmt.Sprintf("%s|%s|%s", overlapStart.Format(time.RFC3339), overlapEnd.Format(time.RFC3339), strings.Join(calendarsInvolved, ","))

				if !seen[key] {
					seen[key] = true
					conflicts = append(conflicts, conflict{
						Start:     overlapStart.Format(time.RFC3339),
						End:       overlapEnd.Format(time.RFC3339),
						Calendars: calendarsInvolved,
					})
				}
			}
		}
	}

	return conflicts
}
