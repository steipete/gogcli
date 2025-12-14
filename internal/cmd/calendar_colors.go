package cmd

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/steipete/gogcli/internal/outfmt"
	"github.com/steipete/gogcli/internal/ui"
)

func newCalendarColorsCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "colors",
		Short: "List available event and calendar colors",
		Long: `List available event and calendar colors with their IDs.

Event colors can be used when creating or updating events.
Calendar colors can be used when creating or updating calendars.`,
		Args: cobra.NoArgs,
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

			// Table output
			if len(colors.Event) == 0 && len(colors.Calendar) == 0 {
				u.Err().Println("No colors available")
				return nil
			}

			// Event colors
			if len(colors.Event) > 0 {
				fmt.Println("EVENT COLORS:")
				tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
				fmt.Fprintln(tw, "ID\tBACKGROUND\tFOREGROUND")

				// Sort color IDs numerically
				ids := make([]int, 0, len(colors.Event))
				for id := range colors.Event {
					if num, err := strconv.Atoi(id); err == nil {
						ids = append(ids, num)
					}
				}
				sort.Ints(ids)

				for _, num := range ids {
					id := strconv.Itoa(num)
					c := colors.Event[id]
					fmt.Fprintf(tw, "%s\t%s\t%s\n", id, c.Background, c.Foreground)
				}
				_ = tw.Flush()
				fmt.Println()
			}

			// Calendar colors
			if len(colors.Calendar) > 0 {
				fmt.Println("CALENDAR COLORS:")
				tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
				fmt.Fprintln(tw, "ID\tBACKGROUND\tFOREGROUND")

				// Sort color IDs numerically
				ids := make([]int, 0, len(colors.Calendar))
				for id := range colors.Calendar {
					if num, err := strconv.Atoi(id); err == nil {
						ids = append(ids, num)
					}
				}
				sort.Ints(ids)

				for _, num := range ids {
					id := strconv.Itoa(num)
					c := colors.Calendar[id]
					fmt.Fprintf(tw, "%s\t%s\t%s\n", id, c.Background, c.Foreground)
				}
				_ = tw.Flush()
			}

			return nil
		},
	}
}
