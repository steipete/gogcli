package cmd

import (
	"errors"
	"os"

	"github.com/spf13/cobra"
	"github.com/steipete/gogcli/internal/outfmt"
	"github.com/steipete/gogcli/internal/ui"
	"google.golang.org/api/gmail/v1"
)

func newGmailBatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "batch",
		Short: "Batch operations on messages",
	}

	cmd.AddCommand(newGmailBatchDeleteCmd(flags))
	cmd.AddCommand(newGmailBatchModifyCmd(flags))
	return cmd
}

func newGmailBatchDeleteCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <messageIds...>",
		Short: "Permanently delete multiple messages",
		Long: `Permanently delete multiple messages. This action cannot be undone.
The messages are immediately and permanently deleted, not moved to trash.`,
		Args: cobra.MinimumNArgs(1),
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

			err = svc.Users.Messages.BatchDelete("me", &gmail.BatchDeleteMessagesRequest{
				Ids: args,
			}).Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{
					"deleted": args,
					"count":   len(args),
				})
			}

			u.Out().Printf("Deleted %d messages", len(args))
			return nil
		},
	}
}

func newGmailBatchModifyCmd(flags *rootFlags) *cobra.Command {
	var add string
	var remove string

	cmd := &cobra.Command{
		Use:   "modify <messageIds...>",
		Short: "Modify labels on multiple messages",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}

			addLabels := splitCSV(add)
			removeLabels := splitCSV(remove)
			if len(addLabels) == 0 && len(removeLabels) == 0 {
				return errors.New("must specify --add and/or --remove")
			}

			svc, err := newGmailService(cmd.Context(), account)
			if err != nil {
				return err
			}

			idMap, err := fetchLabelNameToID(svc)
			if err != nil {
				return err
			}

			addIDs := resolveLabelIDs(addLabels, idMap)
			removeIDs := resolveLabelIDs(removeLabels, idMap)

			err = svc.Users.Messages.BatchModify("me", &gmail.BatchModifyMessagesRequest{
				Ids:            args,
				AddLabelIds:    addIDs,
				RemoveLabelIds: removeIDs,
			}).Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{
					"modified":      args,
					"count":         len(args),
					"addedLabels":   addIDs,
					"removedLabels": removeIDs,
				})
			}

			u.Out().Printf("Modified %d messages", len(args))
			return nil
		},
	}

	cmd.Flags().StringVar(&add, "add", "", "Labels to add (comma-separated, name or ID)")
	cmd.Flags().StringVar(&remove, "remove", "", "Labels to remove (comma-separated, name or ID)")
	return cmd
}
