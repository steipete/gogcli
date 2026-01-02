package cmd

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"google.golang.org/api/gmail/v1"

	"github.com/steipete/gogcli/internal/outfmt"
	"github.com/steipete/gogcli/internal/ui"
)

type GmailSendCmd struct {
	To               string   `name:"to" help:"Recipients (comma-separated, required)"`
	Cc               string   `name:"cc" help:"CC recipients (comma-separated)"`
	Bcc              string   `name:"bcc" help:"BCC recipients (comma-separated)"`
	Subject          string   `name:"subject" help:"Subject (required)"`
	Body             string   `name:"body" help:"Body (plain text; required unless --body-html is set)"`
	BodyHTML         string   `name:"body-html" help:"Body (HTML; optional)"`
	ReplyToMessageID string   `name:"reply-to-message-id" help:"Reply to Gmail message ID (sets In-Reply-To/References and thread)"`
	ReplyTo          string   `name:"reply-to" help:"Reply-To header address"`
	Attach           []string `name:"attach" help:"Attachment file path (repeatable)"`
	From             string   `name:"from" help:"Send from this email address (must be a verified send-as alias)"`
}

func (c *GmailSendCmd) Run(ctx context.Context, flags *RootFlags) error {
	u := ui.FromContext(ctx)
	account, err := requireAccount(flags)
	if err != nil {
		return err
	}

	if strings.TrimSpace(c.To) == "" || strings.TrimSpace(c.Subject) == "" {
		return usage("required: --to, --subject")
	}
	if strings.TrimSpace(c.Body) == "" && strings.TrimSpace(c.BodyHTML) == "" {
		return usage("required: --body or --body-html")
	}

	svc, err := newGmailService(ctx, account)
	if err != nil {
		return err
	}

	// Determine the From address
	fromAddr := account
	if strings.TrimSpace(c.From) != "" {
		// Validate that this is a configured send-as alias
		var sa *gmail.SendAs
		sa, err = svc.Users.Settings.SendAs.Get("me", c.From).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("invalid --from address %q: %w", c.From, err)
		}
		if sa.VerificationStatus != gmailVerificationAccepted {
			return fmt.Errorf("--from address %q is not verified (status: %s)", c.From, sa.VerificationStatus)
		}
		fromAddr = c.From
		// Include display name if set
		if sa.DisplayName != "" {
			fromAddr = sa.DisplayName + " <" + c.From + ">"
		}
	}

	inReplyTo, references, threadID, err := replyHeaders(ctx, svc, c.ReplyToMessageID)
	if err != nil {
		return err
	}

	atts := make([]mailAttachment, 0, len(c.Attach))
	for _, p := range c.Attach {
		atts = append(atts, mailAttachment{Path: p})
	}

	raw, err := buildRFC822(mailOptions{
		From:        fromAddr,
		To:          splitCSV(c.To),
		Cc:          splitCSV(c.Cc),
		Bcc:         splitCSV(c.Bcc),
		ReplyTo:     c.ReplyTo,
		Subject:     c.Subject,
		Body:        c.Body,
		BodyHTML:    c.BodyHTML,
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

	sent, err := svc.Users.Messages.Send("me", msg).Context(ctx).Do()
	if err != nil {
		return err
	}
	if outfmt.IsJSON(ctx) {
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
}

func replyHeaders(ctx context.Context, svc *gmail.Service, replyToMessageID string) (inReplyTo string, references string, threadID string, err error) {
	replyToMessageID = strings.TrimSpace(replyToMessageID)
	if replyToMessageID == "" {
		return "", "", "", nil
	}
	msg, err := svc.Users.Messages.Get("me", replyToMessageID).
		Format("metadata").
		MetadataHeaders("Message-ID", "Message-Id", "References", "In-Reply-To").
		Context(ctx).
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
