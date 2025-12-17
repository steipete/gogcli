package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/steipete/gogcli/internal/googleapi"
	"github.com/steipete/gogcli/internal/outfmt"
	"github.com/steipete/gogcli/internal/ui"
	"google.golang.org/api/sheets/v4"
)

var newSheetsService = googleapi.NewSheets

// cleanRange removes shell escape sequences from range arguments.
// Some shells escape ! to \! (bash history expansion), which breaks Google Sheets API calls.
func cleanRange(r string) string {
	return strings.ReplaceAll(r, `\!`, "!")
}

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
			rangeSpec := cleanRange(args[1])

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
	var valueInputOption string
	var jsonValues string

	cmd := &cobra.Command{
		Use:   "update <spreadsheetId> <range> [values...]",
		Short: "Update values in a range",
		Long: `Update values in a specified range.

Values can be provided as:
1. Command line args (comma-separated rows, pipe-separated cells):
   gog sheets update ID 'A1' 'a|b|c,d|e|f'  (2 rows, 3 cols each)

2. JSON via --json flag:
   gog sheets update ID 'A1' --json '[["a","b"],["c","d"]]'

Examples:
  gog sheets update 1BxiMVs... 'Sheet1!A1' 'Hello|World'
  gog sheets update 1BxiMVs... 'Sheet1!A1:B2' --json '[["a","b"],["c","d"]]'`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}

			spreadsheetID := args[0]
			rangeSpec := cleanRange(args[1])

			var values [][]interface{}

			if jsonValues != "" {
				if unmarshalErr := json.Unmarshal([]byte(jsonValues), &values); unmarshalErr != nil {
					return fmt.Errorf("invalid JSON values: %w", unmarshalErr)
				}
			} else if len(args) > 2 {
				// Parse comma-separated rows, pipe-separated cells
				rawValues := strings.Join(args[2:], " ")
				rows := strings.Split(rawValues, ",")
				for _, row := range rows {
					cells := strings.Split(strings.TrimSpace(row), "|")
					rowData := make([]interface{}, len(cells))
					for i, cell := range cells {
						rowData[i] = strings.TrimSpace(cell)
					}
					values = append(values, rowData)
				}
			} else {
				return fmt.Errorf("provide values as args or via --json")
			}

			svc, err := newSheetsService(cmd.Context(), account)
			if err != nil {
				return err
			}

			vr := &sheets.ValueRange{
				Values: values,
			}

			call := svc.Spreadsheets.Values.Update(spreadsheetID, rangeSpec, vr)
			if valueInputOption == "" {
				valueInputOption = "USER_ENTERED"
			}
			call = call.ValueInputOption(valueInputOption)

			resp, err := call.Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{
					"updatedRange":   resp.UpdatedRange,
					"updatedRows":    resp.UpdatedRows,
					"updatedColumns": resp.UpdatedColumns,
					"updatedCells":   resp.UpdatedCells,
				})
			}

			u.Out().Printf("Updated %d cells in %s", resp.UpdatedCells, resp.UpdatedRange)
			return nil
		},
	}

	cmd.Flags().StringVar(&valueInputOption, "input", "USER_ENTERED", "Value input option: RAW or USER_ENTERED")
	cmd.Flags().StringVar(&jsonValues, "json", "", "Values as JSON 2D array")
	return cmd
}

func newSheetsAppendCmd(flags *rootFlags) *cobra.Command {
	var valueInputOption string
	var insertDataOption string
	var jsonValues string

	cmd := &cobra.Command{
		Use:   "append <spreadsheetId> <range> [values...]",
		Short: "Append values to a range",
		Long: `Append values after the last row with data in a range.

Values format same as 'update' command.

Examples:
  gog sheets append 1BxiMVs... 'Sheet1!A:C' 'val1|val2|val3'
  gog sheets append 1BxiMVs... 'Sheet1!A:C' --json '[["a","b","c"]]'`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}

			spreadsheetID := args[0]
			rangeSpec := cleanRange(args[1])

			var values [][]interface{}

			if jsonValues != "" {
				if unmarshalErr := json.Unmarshal([]byte(jsonValues), &values); unmarshalErr != nil {
					return fmt.Errorf("invalid JSON values: %w", unmarshalErr)
				}
			} else if len(args) > 2 {
				rawValues := strings.Join(args[2:], " ")
				rows := strings.Split(rawValues, ",")
				for _, row := range rows {
					cells := strings.Split(strings.TrimSpace(row), "|")
					rowData := make([]interface{}, len(cells))
					for i, cell := range cells {
						rowData[i] = strings.TrimSpace(cell)
					}
					values = append(values, rowData)
				}
			} else {
				return fmt.Errorf("provide values as args or via --json")
			}

			svc, err := newSheetsService(cmd.Context(), account)
			if err != nil {
				return err
			}

			vr := &sheets.ValueRange{
				Values: values,
			}

			call := svc.Spreadsheets.Values.Append(spreadsheetID, rangeSpec, vr)
			if valueInputOption == "" {
				valueInputOption = "USER_ENTERED"
			}
			call = call.ValueInputOption(valueInputOption)
			if insertDataOption != "" {
				call = call.InsertDataOption(insertDataOption)
			}

			resp, err := call.Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{
					"updatedRange":   resp.Updates.UpdatedRange,
					"updatedRows":    resp.Updates.UpdatedRows,
					"updatedColumns": resp.Updates.UpdatedColumns,
					"updatedCells":   resp.Updates.UpdatedCells,
				})
			}

			u.Out().Printf("Appended %d cells to %s", resp.Updates.UpdatedCells, resp.Updates.UpdatedRange)
			return nil
		},
	}

	cmd.Flags().StringVar(&valueInputOption, "input", "USER_ENTERED", "Value input option: RAW or USER_ENTERED")
	cmd.Flags().StringVar(&insertDataOption, "insert", "", "Insert data option: OVERWRITE or INSERT_ROWS")
	cmd.Flags().StringVar(&jsonValues, "json", "", "Values as JSON 2D array")
	return cmd
}

func newSheetsClearCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "clear <spreadsheetId> <range>",
		Short: "Clear values in a range",
		Long:  "Clear all values in a specified range (keeps formatting).",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}

			spreadsheetID := args[0]
			rangeSpec := cleanRange(args[1])

			svc, err := newSheetsService(cmd.Context(), account)
			if err != nil {
				return err
			}

			resp, err := svc.Spreadsheets.Values.Clear(spreadsheetID, rangeSpec, &sheets.ClearValuesRequest{}).Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{
					"clearedRange": resp.ClearedRange,
				})
			}

			u.Out().Printf("Cleared %s", resp.ClearedRange)
			return nil
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
	var sheetNames string

	cmd := &cobra.Command{
		Use:   "create <title>",
		Short: "Create a new spreadsheet",
		Long: `Create a new Google Sheets spreadsheet.

Examples:
  gog sheets create "My Spreadsheet"
  gog sheets create "Budget" --sheets "Income,Expenses,Summary"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}

			title := args[0]

			svc, err := newSheetsService(cmd.Context(), account)
			if err != nil {
				return err
			}

			spreadsheet := &sheets.Spreadsheet{
				Properties: &sheets.SpreadsheetProperties{
					Title: title,
				},
			}

			if sheetNames != "" {
				names := strings.Split(sheetNames, ",")
				spreadsheet.Sheets = make([]*sheets.Sheet, len(names))
				for i, name := range names {
					spreadsheet.Sheets[i] = &sheets.Sheet{
						Properties: &sheets.SheetProperties{
							Title: strings.TrimSpace(name),
						},
					}
				}
			}

			resp, err := svc.Spreadsheets.Create(spreadsheet).Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{
					"spreadsheetId":  resp.SpreadsheetId,
					"title":          resp.Properties.Title,
					"spreadsheetUrl": resp.SpreadsheetUrl,
				})
			}

			u.Out().Printf("Created spreadsheet: %s", resp.Properties.Title)
			u.Out().Printf("ID: %s", resp.SpreadsheetId)
			u.Out().Printf("URL: %s", resp.SpreadsheetUrl)
			return nil
		},
	}

	cmd.Flags().StringVar(&sheetNames, "sheets", "", "Comma-separated sheet names to create")
	return cmd
}
