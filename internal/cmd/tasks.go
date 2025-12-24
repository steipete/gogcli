package cmd

import (
	"github.com/spf13/cobra"
	"github.com/steipete/gogcli/internal/googleapi"
)

var newTasksService = googleapi.NewTasks

func newTasksCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tasks",
		Short: "Google Tasks",
	}
	cmd.AddCommand(newTasksListsCmd(flags))
	cmd.AddCommand(newTasksListCmd(flags))
	cmd.AddCommand(newTasksAddCmd(flags))
	cmd.AddCommand(newTasksUpdateCmd(flags))
	cmd.AddCommand(newTasksDoneCmd(flags))
	cmd.AddCommand(newTasksUndoCmd(flags))
	cmd.AddCommand(newTasksDeleteCmd(flags))
	cmd.AddCommand(newTasksClearCmd(flags))
	return cmd
}
