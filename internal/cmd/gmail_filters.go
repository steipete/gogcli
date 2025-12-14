package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/steipete/gogcli/internal/outfmt"
	"github.com/steipete/gogcli/internal/ui"
	"google.golang.org/api/gmail/v1"
)

func newGmailFiltersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "filters",
		Short: "Manage email filters",
	}

	cmd.AddCommand(newGmailFiltersListCmd(flags))
	cmd.AddCommand(newGmailFiltersGetCmd(flags))
	cmd.AddCommand(newGmailFiltersCreateCmd(flags))
	cmd.AddCommand(newGmailFiltersDeleteCmd(flags))
	return cmd
}

func newGmailFiltersListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all email filters",
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

			resp, err := svc.Users.Settings.Filters.List("me").Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"filters": resp.Filter})
			}

			if len(resp.Filter) == 0 {
				u.Err().Println("No filters")
				return nil
			}

			tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tFROM\tTO\tSUBJECT\tQUERY")
			for _, f := range resp.Filter {
				criteria := f.Criteria
				from := ""
				to := ""
				subject := ""
				query := ""
				if criteria != nil {
					from = criteria.From
					to = criteria.To
					subject = criteria.Subject
					query = criteria.Query
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
					f.Id,
					sanitizeTab(from),
					sanitizeTab(to),
					sanitizeTab(subject),
					sanitizeTab(query))
			}
			_ = tw.Flush()
			return nil
		},
	}
}

func newGmailFiltersGetCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get <filterId>",
		Short: "Get a specific filter",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}

			svc, err := newGmailService(cmd.Context(), account)
			if err != nil {
				return err
			}

			filterID := args[0]
			filter, err := svc.Users.Settings.Filters.Get("me", filterID).Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"filter": filter})
			}

			u.Out().Printf("id\t%s", filter.Id)
			if filter.Criteria != nil {
				c := filter.Criteria
				if c.From != "" {
					u.Out().Printf("from\t%s", c.From)
				}
				if c.To != "" {
					u.Out().Printf("to\t%s", c.To)
				}
				if c.Subject != "" {
					u.Out().Printf("subject\t%s", c.Subject)
				}
				if c.Query != "" {
					u.Out().Printf("query\t%s", c.Query)
				}
				if c.HasAttachment {
					u.Out().Printf("has_attachment\ttrue")
				}
				if c.NegatedQuery != "" {
					u.Out().Printf("negated_query\t%s", c.NegatedQuery)
				}
				if c.Size != 0 {
					u.Out().Printf("size\t%d", c.Size)
				}
				if c.SizeComparison != "" {
					u.Out().Printf("size_comparison\t%s", c.SizeComparison)
				}
				if c.ExcludeChats {
					u.Out().Printf("exclude_chats\ttrue")
				}
			}
			if filter.Action != nil {
				a := filter.Action
				if len(a.AddLabelIds) > 0 {
					u.Out().Printf("add_label_ids\t%s", strings.Join(a.AddLabelIds, ","))
				}
				if len(a.RemoveLabelIds) > 0 {
					u.Out().Printf("remove_label_ids\t%s", strings.Join(a.RemoveLabelIds, ","))
				}
				if a.Forward != "" {
					u.Out().Printf("forward\t%s", a.Forward)
				}
			}
			return nil
		},
	}
}

func newGmailFiltersCreateCmd(flags *rootFlags) *cobra.Command {
	var from string
	var to string
	var subject string
	var query string
	var hasAttachment bool
	var addLabel string
	var removeLabel string
	var archive bool
	var markRead bool
	var star bool
	var forward string
	var trash bool
	var neverSpam bool
	var important bool

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new email filter",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}

			// Validate that at least one criteria is specified
			if from == "" && to == "" && subject == "" && query == "" && !hasAttachment {
				return errors.New("must specify at least one criteria flag (--from, --to, --subject, --query, or --has-attachment)")
			}

			// Validate that at least one action is specified
			if addLabel == "" && removeLabel == "" && !archive && !markRead && !star && forward == "" && !trash && !neverSpam && !important {
				return errors.New("must specify at least one action flag (--add-label, --remove-label, --archive, --mark-read, --star, --forward, --trash, --never-spam, or --important)")
			}

			svc, err := newGmailService(cmd.Context(), account)
			if err != nil {
				return err
			}

			// Build filter criteria
			criteria := &gmail.FilterCriteria{}
			if from != "" {
				criteria.From = from
			}
			if to != "" {
				criteria.To = to
			}
			if subject != "" {
				criteria.Subject = subject
			}
			if query != "" {
				criteria.Query = query
			}
			if hasAttachment {
				criteria.HasAttachment = true
			}

			// Build filter actions
			action := &gmail.FilterAction{}

			// Resolve label names to IDs for add/remove operations
			var labelMap map[string]string
			if addLabel != "" || removeLabel != "" {
				labelMap, err = fetchLabelNameToID(svc)
				if err != nil {
					return err
				}
			}

			if addLabel != "" {
				addLabels := splitCSV(addLabel)
				addIDs := resolveLabelIDs(addLabels, labelMap)
				action.AddLabelIds = addIDs
			}

			if removeLabel != "" {
				removeLabels := splitCSV(removeLabel)
				removeIDs := resolveLabelIDs(removeLabels, labelMap)
				action.RemoveLabelIds = removeIDs
			}

			if archive {
				// Archive means remove from INBOX
				if action.RemoveLabelIds == nil {
					action.RemoveLabelIds = []string{}
				}
				action.RemoveLabelIds = append(action.RemoveLabelIds, "INBOX")
			}

			if markRead {
				// Mark as read means remove UNREAD label
				if action.RemoveLabelIds == nil {
					action.RemoveLabelIds = []string{}
				}
				action.RemoveLabelIds = append(action.RemoveLabelIds, "UNREAD")
			}

			if star {
				// Star means add STARRED label
				if action.AddLabelIds == nil {
					action.AddLabelIds = []string{}
				}
				action.AddLabelIds = append(action.AddLabelIds, "STARRED")
			}

			if forward != "" {
				action.Forward = forward
			}

			if trash {
				// Trash means add TRASH label
				if action.AddLabelIds == nil {
					action.AddLabelIds = []string{}
				}
				action.AddLabelIds = append(action.AddLabelIds, "TRASH")
			}

			if neverSpam {
				// Never spam means remove SPAM label
				if action.RemoveLabelIds == nil {
					action.RemoveLabelIds = []string{}
				}
				action.RemoveLabelIds = append(action.RemoveLabelIds, "SPAM")
			}

			if important {
				// Important means add IMPORTANT label
				if action.AddLabelIds == nil {
					action.AddLabelIds = []string{}
				}
				action.AddLabelIds = append(action.AddLabelIds, "IMPORTANT")
			}

			filter := &gmail.Filter{
				Criteria: criteria,
				Action:   action,
			}

			created, err := svc.Users.Settings.Filters.Create("me", filter).Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"filter": created})
			}

			u.Out().Println("Filter created successfully")
			u.Out().Printf("id\t%s", created.Id)
			if created.Criteria != nil {
				c := created.Criteria
				if c.From != "" {
					u.Out().Printf("from\t%s", c.From)
				}
				if c.To != "" {
					u.Out().Printf("to\t%s", c.To)
				}
				if c.Subject != "" {
					u.Out().Printf("subject\t%s", c.Subject)
				}
				if c.Query != "" {
					u.Out().Printf("query\t%s", c.Query)
				}
			}
			return nil
		},
	}

	// Criteria flags
	cmd.Flags().StringVar(&from, "from", "", "Match messages from this sender")
	cmd.Flags().StringVar(&to, "to", "", "Match messages to this recipient")
	cmd.Flags().StringVar(&subject, "subject", "", "Match messages with this subject")
	cmd.Flags().StringVar(&query, "query", "", "Advanced Gmail search query for matching")
	cmd.Flags().BoolVar(&hasAttachment, "has-attachment", false, "Match messages with attachments")

	// Action flags
	cmd.Flags().StringVar(&addLabel, "add-label", "", "Label(s) to add to matching messages (comma-separated, name or ID)")
	cmd.Flags().StringVar(&removeLabel, "remove-label", "", "Label(s) to remove from matching messages (comma-separated, name or ID)")
	cmd.Flags().BoolVar(&archive, "archive", false, "Archive matching messages (skip inbox)")
	cmd.Flags().BoolVar(&markRead, "mark-read", false, "Mark matching messages as read")
	cmd.Flags().BoolVar(&star, "star", false, "Star matching messages")
	cmd.Flags().StringVar(&forward, "forward", "", "Forward to this email address")
	cmd.Flags().BoolVar(&trash, "trash", false, "Move matching messages to trash")
	cmd.Flags().BoolVar(&neverSpam, "never-spam", false, "Never mark as spam")
	cmd.Flags().BoolVar(&important, "important", false, "Mark as important")

	return cmd
}

func newGmailFiltersDeleteCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <filterId>",
		Short: "Delete a filter",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}

			svc, err := newGmailService(cmd.Context(), account)
			if err != nil {
				return err
			}

			filterID := args[0]
			err = svc.Users.Settings.Filters.Delete("me", filterID).Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{
					"success":  true,
					"filterId": filterID,
				})
			}

			u.Out().Printf("Filter %s deleted successfully", filterID)
			return nil
		},
	}
}
