package cmd

import (
	"github.com/spf13/cobra"
	"github.com/steipete/gogcli/internal/googleapi"
)

var newSheetsService = googleapi.NewSheets

func newSheetsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sheets",
		Short: "Google Sheets",
	}
	cmd.AddCommand(newSheetsGetCmd(flags))
	cmd.AddCommand(newSheetsUpdateCmd(flags))
	cmd.AddCommand(newSheetsAppendCmd(flags))
	cmd.AddCommand(newSheetsClearCmd(flags))
	cmd.AddCommand(newSheetsMetadataCmd(flags))
	cmd.AddCommand(newSheetsCreateCmd(flags))
	return cmd
}

func newSheetsGetCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get <spreadsheetId> <range>",
		Short: "Get values from a range",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil // Placeholder
		},
	}
}

func newSheetsUpdateCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "update <spreadsheetId> <range>",
		Short: "Update values in a range",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil // Placeholder
		},
	}
}

func newSheetsAppendCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "append <spreadsheetId> <range>",
		Short: "Append values to a range",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil // Placeholder
		},
	}
}

func newSheetsClearCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "clear <spreadsheetId> <range>",
		Short: "Clear values in a range",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil // Placeholder
		},
	}
}

func newSheetsMetadataCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "metadata <spreadsheetId>",
		Short: "Get spreadsheet metadata",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil // Placeholder
		},
	}
}

func newSheetsCreateCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "create <title>",
		Short: "Create a new spreadsheet",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil // Placeholder
		},
	}
}
