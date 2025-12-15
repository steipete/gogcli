package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/steipete/gogcli/internal/googleapi"
	"github.com/steipete/gogcli/internal/outfmt"
	"github.com/steipete/gogcli/internal/ui"
	"google.golang.org/api/keep/v1"
)

var newKeepService = googleapi.NewKeep

func newKeepCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keep",
		Short: "Google Keep",
	}
	cmd.AddCommand(newKeepListCmd(flags))
	cmd.AddCommand(newKeepGetCmd(flags))
	cmd.AddCommand(newKeepCreateCmd(flags))
	cmd.AddCommand(newKeepDeleteCmd(flags))
	return cmd
}

func newKeepListCmd(flags *rootFlags) *cobra.Command {
	var pageSize int64
	var pageToken string
	var filter string

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List Keep notes",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}

			svc, err := newKeepService(cmd.Context(), account)
			if err != nil {
				return err
			}

			call := svc.Notes.List().PageSize(pageSize).PageToken(strings.TrimSpace(pageToken))
			if strings.TrimSpace(filter) != "" {
				call = call.Filter(strings.TrimSpace(filter))
			}
			resp, err := call.Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{
					"notes":         resp.Notes,
					"nextPageToken": resp.NextPageToken,
				})
			}

			if len(resp.Notes) == 0 {
				u.Err().Println("No notes")
				return nil
			}

			tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "NAME\tTITLE\tUPDATED")
			for _, n := range resp.Notes {
				fmt.Fprintf(tw, "%s\t%s\t%s\n", strings.TrimSpace(n.Name), strings.TrimSpace(n.Title), strings.TrimSpace(n.UpdateTime))
			}
			_ = tw.Flush()

			if resp.NextPageToken != "" {
				u.Err().Printf("# Next page: --page %s", resp.NextPageToken)
			}
			return nil
		},
	}

	cmd.Flags().Int64Var(&pageSize, "page-size", 20, "Max results (max allowed: 100)")
	cmd.Flags().StringVar(&pageToken, "page", "", "Page token")
	cmd.Flags().StringVar(&filter, "filter", "", "Filter expression")
	return cmd
}

func newKeepGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <noteName>",
		Short: "Get a Keep note",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			noteName := strings.TrimSpace(args[0])
			if noteName == "" {
				return errors.New("empty noteName")
			}

			svc, err := newKeepService(cmd.Context(), account)
			if err != nil {
				return err
			}

			note, err := svc.Notes.Get(noteName).Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"note": note})
			}

			u.Out().Printf("name\t%s", strings.TrimSpace(note.Name))
			if strings.TrimSpace(note.Title) != "" {
				u.Out().Printf("title\t%s", strings.TrimSpace(note.Title))
			}
			if strings.TrimSpace(note.UpdateTime) != "" {
				u.Out().Printf("updated\t%s", strings.TrimSpace(note.UpdateTime))
			}
			text := keepNoteText(note)
			if strings.TrimSpace(text) != "" {
				u.Out().Printf("text\t%s", text)
			}
			return nil
		},
	}
	return cmd
}

func newKeepCreateCmd(flags *rootFlags) *cobra.Command {
	var title string
	var text string

	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a Keep note (text only)",
		Aliases: []string{"add", "new"},
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			if strings.TrimSpace(title) == "" {
				return errors.New("required: --title")
			}
			if strings.TrimSpace(text) == "" {
				return errors.New("required: --text")
			}

			svc, err := newKeepService(cmd.Context(), account)
			if err != nil {
				return err
			}

			note := &keep.Note{
				Title: strings.TrimSpace(title),
				Body: &keep.Section{
					Text: &keep.TextContent{Text: strings.TrimSpace(text)},
				},
			}
			created, err := svc.Notes.Create(note).Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"note": created})
			}
			u.Out().Printf("name\t%s", strings.TrimSpace(created.Name))
			u.Out().Printf("title\t%s", strings.TrimSpace(created.Title))
			if strings.TrimSpace(created.UpdateTime) != "" {
				u.Out().Printf("updated\t%s", strings.TrimSpace(created.UpdateTime))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "Note title (required)")
	cmd.Flags().StringVar(&text, "text", "", "Note text body (required)")
	return cmd
}

func newKeepDeleteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete <noteName>",
		Short:   "Delete a Keep note",
		Aliases: []string{"rm", "del"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			noteName := strings.TrimSpace(args[0])
			if noteName == "" {
				return errors.New("empty noteName")
			}

			svc, err := newKeepService(cmd.Context(), account)
			if err != nil {
				return err
			}

			_, err = svc.Notes.Delete(noteName).Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"deleted": true, "name": noteName})
			}
			u.Out().Printf("deleted\ttrue")
			u.Out().Printf("name\t%s", noteName)
			return nil
		},
	}
	return cmd
}

func keepNoteText(note *keep.Note) string {
	if note == nil || note.Body == nil || note.Body.Text == nil {
		return ""
	}
	return strings.TrimSpace(note.Body.Text.Text)
}
