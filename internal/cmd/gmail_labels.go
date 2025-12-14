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

func newGmailLabelsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "labels",
		Short: "List and modify labels",
	}

	cmd.AddCommand(newGmailLabelsListCmd(flags))
	cmd.AddCommand(newGmailLabelsGetCmd(flags))
	cmd.AddCommand(newGmailLabelsModifyCmd(flags))
	return cmd
}

func newGmailLabelsGetCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get <labelIdOrName>",
		Short: "Get label details (including counts)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}

			svc, err := newGmailService(cmd.Context(), account)
			if err != nil {
				return err
			}

			idMap, err := fetchLabelNameToID(svc)
			if err != nil {
				return err
			}
			raw := strings.TrimSpace(args[0])
			if raw == "" {
				return errors.New("empty label")
			}
			id := raw
			if v, ok := idMap[strings.ToLower(raw)]; ok {
				id = v
			}

			l, err := svc.Users.Labels.Get("me", id).Context(cmd.Context()).Do()
			if err != nil {
				return err
			}
			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"label": l})
			}
			u := ui.FromContext(cmd.Context())
			u.Out().Printf("id\t%s", l.Id)
			u.Out().Printf("name\t%s", l.Name)
			u.Out().Printf("type\t%s", l.Type)
			u.Out().Printf("messages_total\t%d", l.MessagesTotal)
			u.Out().Printf("messages_unread\t%d", l.MessagesUnread)
			u.Out().Printf("threads_total\t%d", l.ThreadsTotal)
			u.Out().Printf("threads_unread\t%d", l.ThreadsUnread)
			return nil
		},
	}
}

func newGmailLabelsListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List labels",
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

			resp, err := svc.Users.Labels.List("me").Context(cmd.Context()).Do()
			if err != nil {
				return err
			}
			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"labels": resp.Labels})
			}
			if len(resp.Labels) == 0 {
				u.Err().Println("No labels")
				return nil
			}

			tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tNAME\tTYPE")
			for _, l := range resp.Labels {
				fmt.Fprintf(tw, "%s\t%s\t%s\n", l.Id, l.Name, l.Type)
			}
			_ = tw.Flush()
			return nil
		},
	}
}

func newGmailLabelsModifyCmd(flags *rootFlags) *cobra.Command {
	var add string
	var remove string

	cmd := &cobra.Command{
		Use:   "modify <threadIds...>",
		Short: "Modify labels on threads",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			threadIDs := args
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

			type result struct {
				ThreadID string `json:"threadId"`
				Success  bool   `json:"success"`
				Error    string `json:"error,omitempty"`
			}
			results := make([]result, 0, len(threadIDs))

			for _, tid := range threadIDs {
				_, err := svc.Users.Threads.Modify("me", tid, &gmail.ModifyThreadRequest{
					AddLabelIds:    addIDs,
					RemoveLabelIds: removeIDs,
				}).Context(cmd.Context()).Do()
				if err != nil {
					results = append(results, result{ThreadID: tid, Success: false, Error: err.Error()})
					if !outfmt.IsJSON(cmd.Context()) {
						u.Err().Errorf("%s: %s", tid, err.Error())
					}
					continue
				}
				results = append(results, result{ThreadID: tid, Success: true})
				if !outfmt.IsJSON(cmd.Context()) {
					u.Out().Printf("%s\tok", tid)
				}
			}
			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"results": results})
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&add, "add", "", "Labels to add (comma-separated, name or ID)")
	cmd.Flags().StringVar(&remove, "remove", "", "Labels to remove (comma-separated, name or ID)")
	return cmd
}

func fetchLabelNameToID(svc *gmail.Service) (map[string]string, error) {
	resp, err := svc.Users.Labels.List("me").Do()
	if err != nil {
		return nil, err
	}
	m := make(map[string]string, len(resp.Labels))
	for _, l := range resp.Labels {
		if l.Id == "" {
			continue
		}
		m[strings.ToLower(l.Id)] = l.Id
		if l.Name != "" {
			m[strings.ToLower(l.Name)] = l.Id
		}
	}
	return m, nil
}

func fetchLabelIDToName(svc *gmail.Service) (map[string]string, error) {
	resp, err := svc.Users.Labels.List("me").Do()
	if err != nil {
		return nil, err
	}
	m := make(map[string]string, len(resp.Labels))
	for _, l := range resp.Labels {
		if l.Id == "" {
			continue
		}
		if l.Name != "" {
			m[l.Id] = l.Name
		} else {
			m[l.Id] = l.Id
		}
	}
	return m, nil
}

func resolveLabelIDs(values []string, nameToID map[string]string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if id, ok := nameToID[strings.ToLower(v)]; ok {
			out = append(out, id)
		} else {
			out = append(out, v)
		}
	}
	return out
}
