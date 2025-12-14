package cmd

import (
	"fmt"
	"net/mail"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/steipete/gogcli/internal/googleapi"
	"github.com/steipete/gogcli/internal/outfmt"
	"github.com/steipete/gogcli/internal/ui"
	"google.golang.org/api/gmail/v1"
)

var newGmailService = googleapi.NewGmail

func newGmailCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gmail",
		Short: "Gmail",
	}
	cmd.AddCommand(newGmailSearchCmd(flags))
	cmd.AddCommand(newGmailThreadCmd(flags))
	cmd.AddCommand(newGmailURLCmd(flags))
	cmd.AddCommand(newGmailLabelsCmd(flags))
	cmd.AddCommand(newGmailSendCmd(flags))
	cmd.AddCommand(newGmailDraftsCmd(flags))
	cmd.AddCommand(newGmailSendAsCmd(flags))
	cmd.AddCommand(newGmailBatchCmd(flags))
	cmd.AddCommand(newGmailVacationCmd(flags))
	cmd.AddCommand(newGmailFiltersCmd(flags))
	return cmd
}

func newGmailSearchCmd(flags *rootFlags) *cobra.Command {
	var max int64
	var page string

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search threads using Gmail query syntax",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			query := strings.Join(args, " ")

			svc, err := newGmailService(cmd.Context(), account)
			if err != nil {
				return err
			}

			resp, err := svc.Users.Threads.List("me").
				Q(query).
				MaxResults(max).
				PageToken(page).
				Do()
			if err != nil {
				return err
			}

			idToName, err := fetchLabelIDToName(svc)
			if err != nil {
				return err
			}

			type item struct {
				ID      string   `json:"id"`
				Date    string   `json:"date,omitempty"`
				From    string   `json:"from,omitempty"`
				Subject string   `json:"subject,omitempty"`
				Labels  []string `json:"labels,omitempty"`
			}
			items := make([]item, 0, len(resp.Threads))

			tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			for _, t := range resp.Threads {
				if t.Id == "" {
					continue
				}
				thread, err := svc.Users.Threads.Get("me", t.Id).
					Format("metadata").
					MetadataHeaders("From", "Subject", "Date").
					Do()
				if err != nil {
					return err
				}
				msg := firstMessage(thread)
				date := ""
				from := ""
				subject := ""
				var labels []string
				if msg != nil {
					date = formatGmailDate(headerValue(msg.Payload, "Date"))
					from = headerValue(msg.Payload, "From")
					subject = headerValue(msg.Payload, "Subject")
					if len(msg.LabelIds) > 0 {
						names := make([]string, 0, len(msg.LabelIds))
						for _, id := range msg.LabelIds {
							if n, ok := idToName[id]; ok {
								names = append(names, n)
							} else {
								names = append(names, id)
							}
						}
						labels = names
					}
				}

				items = append(items, item{
					ID:      t.Id,
					Date:    date,
					From:    sanitizeTab(from),
					Subject: sanitizeTab(subject),
					Labels:  labels,
				})
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{
					"threads":       items,
					"nextPageToken": resp.NextPageToken,
				})
			}

			if len(items) == 0 {
				u.Err().Println("No results")
				return nil
			}

			fmt.Fprintln(tw, "ID\tDATE\tFROM\tSUBJECT\tLABELS")
			for _, it := range items {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", it.ID, it.Date, it.From, it.Subject, strings.Join(it.Labels, ","))
			}
			_ = tw.Flush()

			if resp.NextPageToken != "" {
				u.Err().Printf("# Next page: --page %s", resp.NextPageToken)
			}
			return nil
		},
	}

	cmd.Flags().Int64Var(&max, "max", 10, "Max results")
	cmd.Flags().StringVar(&page, "page", "", "Page token")
	return cmd
}

func firstMessage(t *gmail.Thread) *gmail.Message {
	if t == nil || len(t.Messages) == 0 {
		return nil
	}
	return t.Messages[0]
}

func headerValue(p *gmail.MessagePart, name string) string {
	if p == nil {
		return ""
	}
	for _, h := range p.Headers {
		if strings.EqualFold(h.Name, name) {
			return h.Value
		}
	}
	return ""
}

func formatGmailDate(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if t, err := mailParseDate(raw); err == nil {
		return t.Format("2006-01-02 15:04")
	}
	return raw
}

func sanitizeTab(s string) string {
	return strings.ReplaceAll(s, "\t", " ")
}

func mailParseDate(s string) (time.Time, error) {
	// net/mail has the most compatible Date parser, but we keep this isolated for easier tests/mocks later.
	return mail.ParseDate(s)
}
