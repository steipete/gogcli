package cmd

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steipete/gogcli/internal/outfmt"
	"github.com/steipete/gogcli/internal/ui"
	"google.golang.org/api/gmail/v1"
)

func newGmailSendCmd(flags *rootFlags) *cobra.Command {
	var to string
	var cc string
	var bcc string
	var subject string
	var body string
	var replyTo string
	var attach []string
	var from string

	cmd := &cobra.Command{
		Use:   "send",
		Short: "Send an email",
		Long: `Send an email. Use --from to send from a configured send-as alias.

To see available send-as aliases: gogcli gmail sendas list`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			if strings.TrimSpace(to) == "" || strings.TrimSpace(subject) == "" || strings.TrimSpace(body) == "" {
				return errors.New("required: --to, --subject, --body")
			}

			svc, err := newGmailService(cmd.Context(), account)
			if err != nil {
				return err
			}

			// Determine the From address
			fromAddr := account
			if strings.TrimSpace(from) != "" {
				// Validate that this is a configured send-as alias
				sa, err := svc.Users.Settings.SendAs.Get("me", from).Do()
				if err != nil {
					return fmt.Errorf("invalid --from address %q: %w", from, err)
				}
				if sa.VerificationStatus != "accepted" {
					return fmt.Errorf("--from address %q is not verified (status: %s)", from, sa.VerificationStatus)
				}
				fromAddr = from
				// Include display name if set
				if sa.DisplayName != "" {
					fromAddr = sa.DisplayName + " <" + from + ">"
				}
			}

			inReplyTo, references, threadID, err := replyHeaders(cmd, svc, replyTo)
			if err != nil {
				return err
			}

			atts := make([]mailAttachment, 0, len(attach))
			for _, p := range attach {
				atts = append(atts, mailAttachment{Path: p})
			}

			raw, err := buildRFC822(mailOptions{
				From:        fromAddr,
				To:          splitCSV(to),
				Cc:          splitCSV(cc),
				Bcc:         splitCSV(bcc),
				Subject:     subject,
				Body:        body,
				InReplyTo:   inReplyTo,
				References:  references,
				Attachments: atts,
			})
			if err != nil {
				return err
			}

			msg := &gmail.Message{
				Raw: base64.RawURLEncoding.EncodeToString(raw),
			}
			if threadID != "" {
				msg.ThreadId = threadID
			}

			sent, err := svc.Users.Messages.Send("me", msg).Do()
			if err != nil {
				return err
			}
			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{
					"messageId": sent.Id,
					"threadId":  sent.ThreadId,
					"from":      fromAddr,
				})
			}
			u.Out().Printf("message_id\t%s", sent.Id)
			if sent.ThreadId != "" {
				u.Out().Printf("thread_id\t%s", sent.ThreadId)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&to, "to", "", "Recipients (comma-separated, required)")
	cmd.Flags().StringVar(&cc, "cc", "", "CC recipients (comma-separated)")
	cmd.Flags().StringVar(&bcc, "bcc", "", "BCC recipients (comma-separated)")
	cmd.Flags().StringVar(&subject, "subject", "", "Subject (required)")
	cmd.Flags().StringVar(&body, "body", "", "Body (required)")
	cmd.Flags().StringVar(&replyTo, "reply-to", "", "Reply to message ID (sets In-Reply-To/References and thread)")
	cmd.Flags().StringSliceVar(&attach, "attach", nil, "Attachment file path (repeatable)")
	cmd.Flags().StringVar(&from, "from", "", "Send from this email address (must be a verified send-as alias)")
	return cmd
}

func replyHeaders(cmd *cobra.Command, svc *gmail.Service, replyToMessageID string) (inReplyTo string, references string, threadID string, err error) {
	replyToMessageID = strings.TrimSpace(replyToMessageID)
	if replyToMessageID == "" {
		return "", "", "", nil
	}
	msg, err := svc.Users.Messages.Get("me", replyToMessageID).
		Format("metadata").
		MetadataHeaders("Message-ID", "Message-Id", "References", "In-Reply-To").
		Context(cmd.Context()).
		Do()
	if err != nil {
		return "", "", "", err
	}
	threadID = msg.ThreadId
	// Prefer Message-ID and References from the original message.
	messageID := headerValue(msg.Payload, "Message-ID")
	if messageID == "" {
		messageID = headerValue(msg.Payload, "Message-Id")
	}
	inReplyTo = messageID
	references = strings.TrimSpace(headerValue(msg.Payload, "References"))
	if references == "" {
		references = messageID
	} else if messageID != "" && !strings.Contains(references, messageID) {
		references = references + " " + messageID
	}
	return inReplyTo, references, threadID, nil
}
