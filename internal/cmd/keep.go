package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	keepapi "google.golang.org/api/keep/v1"

	"github.com/steipete/gogcli/internal/config"
	"github.com/steipete/gogcli/internal/googleapi"
	"github.com/steipete/gogcli/internal/outfmt"
	"github.com/steipete/gogcli/internal/ui"
)

var newKeepService = googleapi.NewKeep
var newKeepServiceWithSA = googleapi.NewKeepWithServiceAccount

type KeepCmd struct {
	ServiceAccount string `name:"service-account" help:"Path to service account JSON file"`
	Impersonate    string `name:"impersonate" help:"Email to impersonate (required with service-account)"`

	List KeepListCmd `cmd:"" default:"withargs" help:"List notes"`
	Get  KeepGetCmd  `cmd:"" name:"get" help:"Get a note"`
}

type KeepListCmd struct {
	Max  int64  `name:"max" help:"Max results" default:"100"`
	Page string `name:"page" help:"Page token"`
}

func (c *KeepListCmd) Run(ctx context.Context, flags *RootFlags, keep *KeepCmd) error {
	u := ui.FromContext(ctx)

	svc, err := getKeepService(ctx, flags, keep)
	if err != nil {
		return err
	}

	call := svc.Notes.List().PageSize(c.Max).PageToken(c.Page)
	resp, err := call.Do()
	if err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{
			"notes":         resp.Notes,
			"nextPageToken": resp.NextPageToken,
		})
	}

	if len(resp.Notes) == 0 {
		u.Err().Println("No notes")
		return nil
	}

	w, flush := tableWriter(ctx)
	defer flush()
	fmt.Fprintln(w, "NAME\tTITLE\tUPDATED")
	for _, n := range resp.Notes {
		title := n.Title
		if title == "" {
			title = noteSnippet(n)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", n.Name, title, n.UpdateTime)
	}
	printNextPageHint(u, resp.NextPageToken)
	return nil
}

func noteSnippet(n *keepapi.Note) string {
	if n.Body == nil || n.Body.Text == nil {
		return "(no content)"
	}
	text := n.Body.Text.Text
	if len(text) > 50 {
		text = text[:50] + "..."
	}
	text = strings.ReplaceAll(text, "\n", " ")
	return text
}

type KeepGetCmd struct {
	NoteID string `arg:"" name:"noteId" help:"Note ID or name (e.g. notes/abc123)"`
}

func (c *KeepGetCmd) Run(ctx context.Context, flags *RootFlags, keep *KeepCmd) error {
	u := ui.FromContext(ctx)

	svc, err := getKeepService(ctx, flags, keep)
	if err != nil {
		return err
	}

	name := c.NoteID
	if !strings.HasPrefix(name, "notes/") {
		name = "notes/" + name
	}

	note, err := svc.Notes.Get(name).Do()
	if err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{"note": note})
	}

	u.Out().Printf("name\t%s", note.Name)
	u.Out().Printf("title\t%s", note.Title)
	u.Out().Printf("created\t%s", note.CreateTime)
	u.Out().Printf("updated\t%s", note.UpdateTime)
	u.Out().Printf("trashed\t%v", note.Trashed)
	if note.Body != nil && note.Body.Text != nil {
		u.Out().Println("")
		u.Out().Println(note.Body.Text.Text)
	}
	if len(note.Attachments) > 0 {
		u.Out().Println("")
		u.Out().Printf("attachments\t%d", len(note.Attachments))
		for _, a := range note.Attachments {
			u.Out().Printf("  %s\t%s", a.Name, a.MimeType)
		}
	}
	return nil
}

func getKeepService(ctx context.Context, flags *RootFlags, keepCmd *KeepCmd) (*keepapi.Service, error) {
	if keepCmd.ServiceAccount != "" {
		if keepCmd.Impersonate == "" {
			return nil, fmt.Errorf("--impersonate is required when using --service-account")
		}
		return newKeepServiceWithSA(ctx, keepCmd.ServiceAccount, keepCmd.Impersonate)
	}

	account, err := requireAccount(flags)
	if err != nil {
		return nil, err
	}

	saPath, err := config.KeepServiceAccountPath(account)
	if err != nil {
		return nil, err
	}

	if _, statErr := os.Stat(saPath); statErr == nil {
		return newKeepServiceWithSA(ctx, saPath, account)
	}

	return newKeepService(ctx, account)
}
