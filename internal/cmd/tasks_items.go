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
	"google.golang.org/api/tasks/v1"
)

func newTasksListCmd(flags *rootFlags) *cobra.Command {
	var max int64
	var page string
	var showCompleted bool
	var showDeleted bool
	var showHidden bool
	var showAssigned bool
	var dueMin string
	var dueMax string
	var completedMin string
	var completedMax string
	var updatedMin string

	cmd := &cobra.Command{
		Use:   "list <tasklistId>",
		Short: "List tasks in a task list",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			tasklistID := strings.TrimSpace(args[0])
			if tasklistID == "" {
				return errors.New("empty tasklistId")
			}

			svc, err := newTasksService(cmd.Context(), account)
			if err != nil {
				return err
			}

			call := svc.Tasks.List(tasklistID).
				MaxResults(max).
				PageToken(page).
				ShowCompleted(showCompleted).
				ShowDeleted(showDeleted).
				ShowHidden(showHidden).
				ShowAssigned(showAssigned)
			if strings.TrimSpace(dueMin) != "" {
				call = call.DueMin(strings.TrimSpace(dueMin))
			}
			if strings.TrimSpace(dueMax) != "" {
				call = call.DueMax(strings.TrimSpace(dueMax))
			}
			if strings.TrimSpace(completedMin) != "" {
				call = call.CompletedMin(strings.TrimSpace(completedMin))
			}
			if strings.TrimSpace(completedMax) != "" {
				call = call.CompletedMax(strings.TrimSpace(completedMax))
			}
			if strings.TrimSpace(updatedMin) != "" {
				call = call.UpdatedMin(strings.TrimSpace(updatedMin))
			}

			resp, err := call.Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{
					"tasks":         resp.Items,
					"nextPageToken": resp.NextPageToken,
				})
			}

			if len(resp.Items) == 0 {
				u.Err().Println("No tasks")
				return nil
			}

			tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tTITLE\tSTATUS\tDUE\tUPDATED")
			for _, t := range resp.Items {
				status := strings.TrimSpace(t.Status)
				if status == "" {
					status = "needsAction"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", t.Id, t.Title, status, strings.TrimSpace(t.Due), strings.TrimSpace(t.Updated))
			}
			_ = tw.Flush()

			if resp.NextPageToken != "" {
				u.Err().Printf("# Next page: --page %s", resp.NextPageToken)
			}
			return nil
		},
	}

	cmd.Flags().Int64Var(&max, "max", 20, "Max results (max allowed: 100)")
	cmd.Flags().StringVar(&page, "page", "", "Page token")

	cmd.Flags().BoolVar(&showCompleted, "show-completed", true, "Include completed tasks (requires --show-hidden for some clients)")
	cmd.Flags().BoolVar(&showDeleted, "show-deleted", false, "Include deleted tasks")
	cmd.Flags().BoolVar(&showHidden, "show-hidden", false, "Include hidden tasks")
	cmd.Flags().BoolVar(&showAssigned, "show-assigned", false, "Include tasks assigned to current user")

	cmd.Flags().StringVar(&dueMin, "due-min", "", "Lower bound for due date filter (RFC3339)")
	cmd.Flags().StringVar(&dueMax, "due-max", "", "Upper bound for due date filter (RFC3339)")
	cmd.Flags().StringVar(&completedMin, "completed-min", "", "Lower bound for completion date filter (RFC3339)")
	cmd.Flags().StringVar(&completedMax, "completed-max", "", "Upper bound for completion date filter (RFC3339)")
	cmd.Flags().StringVar(&updatedMin, "updated-min", "", "Lower bound for updated time filter (RFC3339)")
	return cmd
}

func newTasksAddCmd(flags *rootFlags) *cobra.Command {
	var title string
	var notes string
	var due string
	var parent string
	var previous string

	cmd := &cobra.Command{
		Use:     "add <tasklistId>",
		Short:   "Create a task in a task list",
		Aliases: []string{"create"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			tasklistID := strings.TrimSpace(args[0])
			if tasklistID == "" {
				return errors.New("empty tasklistId")
			}
			if strings.TrimSpace(title) == "" {
				return errors.New("required: --title")
			}

			svc, err := newTasksService(cmd.Context(), account)
			if err != nil {
				return err
			}

			task := &tasks.Task{
				Title: strings.TrimSpace(title),
				Notes: strings.TrimSpace(notes),
				Due:   strings.TrimSpace(due),
			}
			call := svc.Tasks.Insert(tasklistID, task)
			if strings.TrimSpace(parent) != "" {
				call = call.Parent(strings.TrimSpace(parent))
			}
			if strings.TrimSpace(previous) != "" {
				call = call.Previous(strings.TrimSpace(previous))
			}

			created, err := call.Do()
			if err != nil {
				return err
			}
			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"task": created})
			}
			u.Out().Printf("id\t%s", created.Id)
			u.Out().Printf("title\t%s", created.Title)
			if strings.TrimSpace(created.Status) != "" {
				u.Out().Printf("status\t%s", created.Status)
			}
			if strings.TrimSpace(created.Due) != "" {
				u.Out().Printf("due\t%s", created.Due)
			}
			if strings.TrimSpace(created.WebViewLink) != "" {
				u.Out().Printf("link\t%s", created.WebViewLink)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "Task title (required)")
	cmd.Flags().StringVar(&notes, "notes", "", "Task notes/description")
	cmd.Flags().StringVar(&due, "due", "", "Due date/time (RFC3339)")
	cmd.Flags().StringVar(&parent, "parent", "", "Parent task ID (create as subtask)")
	cmd.Flags().StringVar(&previous, "previous", "", "Previous sibling task ID (controls ordering)")
	return cmd
}

func newTasksUpdateCmd(flags *rootFlags) *cobra.Command {
	var title string
	var notes string
	var due string
	var status string

	cmd := &cobra.Command{
		Use:   "update <tasklistId> <taskId>",
		Short: "Update a task",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			tasklistID := strings.TrimSpace(args[0])
			taskID := strings.TrimSpace(args[1])
			if tasklistID == "" {
				return errors.New("empty tasklistId")
			}
			if taskID == "" {
				return errors.New("empty taskId")
			}

			patch := &tasks.Task{}
			changed := false
			if cmd.Flags().Changed("title") {
				patch.Title = strings.TrimSpace(title)
				changed = true
			}
			if cmd.Flags().Changed("notes") {
				patch.Notes = strings.TrimSpace(notes)
				changed = true
			}
			if cmd.Flags().Changed("due") {
				patch.Due = strings.TrimSpace(due)
				changed = true
			}
			if cmd.Flags().Changed("status") {
				patch.Status = strings.TrimSpace(status)
				changed = true
			}
			if !changed {
				return errors.New("no fields to update (set at least one of: --title, --notes, --due, --status)")
			}

			if cmd.Flags().Changed("status") && patch.Status != "" && patch.Status != "needsAction" && patch.Status != "completed" {
				return errors.New("invalid --status (expected needsAction or completed)")
			}

			svc, err := newTasksService(cmd.Context(), account)
			if err != nil {
				return err
			}

			updated, err := svc.Tasks.Patch(tasklistID, taskID, patch).Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"task": updated})
			}
			u.Out().Printf("id\t%s", updated.Id)
			u.Out().Printf("title\t%s", updated.Title)
			if strings.TrimSpace(updated.Status) != "" {
				u.Out().Printf("status\t%s", updated.Status)
			}
			if strings.TrimSpace(updated.Due) != "" {
				u.Out().Printf("due\t%s", updated.Due)
			}
			if strings.TrimSpace(updated.WebViewLink) != "" {
				u.Out().Printf("link\t%s", updated.WebViewLink)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "New title (set empty to clear)")
	cmd.Flags().StringVar(&notes, "notes", "", "New notes (set empty to clear)")
	cmd.Flags().StringVar(&due, "due", "", "New due date/time (RFC3339; set empty to clear)")
	cmd.Flags().StringVar(&status, "status", "", "New status: needsAction|completed (set empty to clear)")
	return cmd
}

func newTasksDoneCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "done <tasklistId> <taskId>",
		Short:   "Mark a task as completed",
		Aliases: []string{"complete"},
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			tasklistID := strings.TrimSpace(args[0])
			taskID := strings.TrimSpace(args[1])
			if tasklistID == "" {
				return errors.New("empty tasklistId")
			}
			if taskID == "" {
				return errors.New("empty taskId")
			}

			svc, err := newTasksService(cmd.Context(), account)
			if err != nil {
				return err
			}

			updated, err := svc.Tasks.Patch(tasklistID, taskID, &tasks.Task{Status: "completed"}).Do()
			if err != nil {
				return err
			}
			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"task": updated})
			}
			u.Out().Printf("id\t%s", updated.Id)
			u.Out().Printf("status\t%s", strings.TrimSpace(updated.Status))
			return nil
		},
	}
	return cmd
}

func newTasksUndoCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "undo <tasklistId> <taskId>",
		Short:   "Mark a task as not completed",
		Aliases: []string{"uncomplete", "undone"},
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			tasklistID := strings.TrimSpace(args[0])
			taskID := strings.TrimSpace(args[1])
			if tasklistID == "" {
				return errors.New("empty tasklistId")
			}
			if taskID == "" {
				return errors.New("empty taskId")
			}

			svc, err := newTasksService(cmd.Context(), account)
			if err != nil {
				return err
			}

			updated, err := svc.Tasks.Patch(tasklistID, taskID, &tasks.Task{Status: "needsAction"}).Do()
			if err != nil {
				return err
			}
			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"task": updated})
			}
			u.Out().Printf("id\t%s", updated.Id)
			u.Out().Printf("status\t%s", strings.TrimSpace(updated.Status))
			return nil
		},
	}
	return cmd
}

func newTasksDeleteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete <tasklistId> <taskId>",
		Short:   "Delete a task",
		Aliases: []string{"rm", "del"},
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			tasklistID := strings.TrimSpace(args[0])
			taskID := strings.TrimSpace(args[1])
			if tasklistID == "" {
				return errors.New("empty tasklistId")
			}
			if taskID == "" {
				return errors.New("empty taskId")
			}

			svc, err := newTasksService(cmd.Context(), account)
			if err != nil {
				return err
			}

			if err := svc.Tasks.Delete(tasklistID, taskID).Do(); err != nil {
				return err
			}
			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{
					"deleted": true,
					"id":      taskID,
				})
			}
			u.Out().Printf("deleted\ttrue")
			u.Out().Printf("id\t%s", taskID)
			return nil
		},
	}
	return cmd
}

func newTasksClearCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clear <tasklistId>",
		Short: "Clear completed tasks from a task list",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			tasklistID := strings.TrimSpace(args[0])
			if tasklistID == "" {
				return errors.New("empty tasklistId")
			}

			svc, err := newTasksService(cmd.Context(), account)
			if err != nil {
				return err
			}

			if err := svc.Tasks.Clear(tasklistID).Do(); err != nil {
				return err
			}
			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{
					"cleared":    true,
					"tasklistId": tasklistID,
				})
			}
			u.Out().Printf("cleared\ttrue")
			u.Out().Printf("tasklistId\t%s", tasklistID)
			return nil
		},
	}
	return cmd
}
