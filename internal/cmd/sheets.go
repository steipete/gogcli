package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/steipete/gogcli/internal/googleapi"
	"github.com/steipete/gogcli/internal/outfmt"
	"github.com/steipete/gogcli/internal/ui"
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
	var majorDimension string
	var valueRenderOption string

	cmd := &cobra.Command{
		Use:   "get <spreadsheetId> <range>",
		Short: "Get values from a range",
		Long:  "Get values from a specified range in a Google Sheets spreadsheet.\nExample: gog sheets get 1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms 'Sheet1!A1:B10'",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}

			spreadsheetID := args[0]
			rangeSpec := args[1]

			svc, err := newSheetsService(cmd.Context(), account)
			if err != nil {
				return err
			}

			call := svc.Spreadsheets.Values.Get(spreadsheetID, rangeSpec)
			if majorDimension != "" {
				call = call.MajorDimension(majorDimension)
			}
			if valueRenderOption != "" {
				call = call.ValueRenderOption(valueRenderOption)
			}

			resp, err := call.Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{
					"range":  resp.Range,
					"values": resp.Values,
				})
			}

			if len(resp.Values) == 0 {
				u.Err().Println("No data found")
				return nil
			}

			tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			for _, row := range resp.Values {
				cells := make([]string, len(row))
				for i, cell := range row {
					cells[i] = fmt.Sprintf("%v", cell)
				}
				fmt.Fprintln(tw, strings.Join(cells, "\t"))
			}
			_ = tw.Flush()
			return nil
		},
	}

	cmd.Flags().StringVar(&majorDimension, "dimension", "", "Major dimension: ROWS or COLUMNS")
	cmd.Flags().StringVar(&valueRenderOption, "render", "", "Value render option: FORMATTED_VALUE, UNFORMATTED_VALUE, or FORMULA")
	return cmd
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
		Long:  "Get metadata about a spreadsheet including title, sheets, and properties.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}

			spreadsheetID := args[0]

			svc, err := newSheetsService(cmd.Context(), account)
			if err != nil {
				return err
			}

			resp, err := svc.Spreadsheets.Get(spreadsheetID).Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{
					"spreadsheetId": resp.SpreadsheetId,
					"title":         resp.Properties.Title,
					"locale":        resp.Properties.Locale,
					"timeZone":      resp.Properties.TimeZone,
					"sheets":        resp.Sheets,
				})
			}

			u.Out().Printf("ID\t%s", resp.SpreadsheetId)
			u.Out().Printf("Title\t%s", resp.Properties.Title)
			u.Out().Printf("Locale\t%s", resp.Properties.Locale)
			u.Out().Printf("TimeZone\t%s", resp.Properties.TimeZone)
			u.Out().Printf("URL\t%s", resp.SpreadsheetUrl)
			u.Out().Println("")
			u.Out().Println("Sheets:")

			tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tTITLE\tROWS\tCOLS")
			for _, sheet := range resp.Sheets {
				props := sheet.Properties
				fmt.Fprintf(tw, "%d\t%s\t%d\t%d\n",
					props.SheetId,
					props.Title,
					props.GridProperties.RowCount,
					props.GridProperties.ColumnCount,
				)
			}
			_ = tw.Flush()
			return nil
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
