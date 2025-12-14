package cmd

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steipete/gogcli/internal/config"
	"github.com/steipete/gogcli/internal/outfmt"
	"github.com/steipete/gogcli/internal/ui"
	"google.golang.org/api/gmail/v1"
)

func newGmailThreadCmd(flags *rootFlags) *cobra.Command {
	var download bool

	cmd := &cobra.Command{
		Use:   "thread <threadId>",
		Short: "Get a thread with all messages (optionally download attachments)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			threadID := args[0]

			svc, err := newGmailService(cmd.Context(), account)
			if err != nil {
				return err
			}

			thread, err := svc.Users.Threads.Get("me", threadID).Format("full").Context(cmd.Context()).Do()
			if err != nil {
				return err
			}
			if outfmt.IsJSON(cmd.Context()) {
				type downloaded struct {
					MessageID     string `json:"messageId"`
					AttachmentID  string `json:"attachmentId"`
					Filename      string `json:"filename"`
					MimeType      string `json:"mimeType,omitempty"`
					Size          int64  `json:"size,omitempty"`
					Path          string `json:"path"`
					Cached        bool   `json:"cached"`
					DownloadError string `json:"error,omitempty"`
				}
				downloadedFiles := make([]downloaded, 0)
				if download && thread != nil {
					d, err := config.EnsureGmailAttachmentsDir()
					if err != nil {
						return err
					}
					for _, msg := range thread.Messages {
						if msg == nil || msg.Id == "" {
							continue
						}
						for _, a := range collectAttachments(msg.Payload) {
							outPath, cached, err := downloadAttachment(cmd, svc, msg.Id, a, d)
							if err != nil {
								return err
							}
							df := downloaded{
								MessageID:    msg.Id,
								AttachmentID: a.AttachmentID,
								Filename:     a.Filename,
								MimeType:     a.MimeType,
								Size:         a.Size,
								Path:         outPath,
								Cached:       cached,
							}
							downloadedFiles = append(downloadedFiles, df)
						}
					}
				}
				return outfmt.WriteJSON(os.Stdout, map[string]any{
					"thread":     thread,
					"downloaded": downloadedFiles,
				})
			}
			if thread == nil || len(thread.Messages) == 0 {
				u.Err().Println("Empty thread")
				return nil
			}

			var attachDir string
			if download {
				d, err := config.EnsureGmailAttachmentsDir()
				if err != nil {
					return err
				}
				attachDir = d
			}

			for _, msg := range thread.Messages {
				if msg == nil {
					continue
				}
				u.Out().Printf("Message: %s", msg.Id)
				u.Out().Printf("From: %s", headerValue(msg.Payload, "From"))
				u.Out().Printf("To: %s", headerValue(msg.Payload, "To"))
				u.Out().Printf("Subject: %s", headerValue(msg.Payload, "Subject"))
				u.Out().Printf("Date: %s", headerValue(msg.Payload, "Date"))
				u.Out().Println("")

				body := bestBodyText(msg.Payload)
				if body != "" {
					u.Out().Println(body)
					u.Out().Println("")
				}

				attachments := collectAttachments(msg.Payload)
				if len(attachments) > 0 {
					u.Out().Println("Attachments:")
					for _, a := range attachments {
						u.Out().Printf("  - %s (%d bytes)", a.Filename, a.Size)
					}
					u.Out().Println("")
				}

				if download && len(attachments) > 0 {
					for _, a := range attachments {
						outPath, cached, err := downloadAttachment(cmd, svc, msg.Id, a, attachDir)
						if err != nil {
							return err
						}
						if cached {
							u.Out().Printf("Cached: %s", outPath)
						} else {
							u.Out().Successf("Saved: %s", outPath)
						}
					}
					u.Out().Println("")
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&download, "download", false, "Download attachments")
	return cmd
}

func newGmailURLCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "url <threadIds...>",
		Short: "Print Gmail web URLs for threads",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			if outfmt.IsJSON(cmd.Context()) {
				urls := make([]map[string]string, 0, len(args))
				for _, id := range args {
					urls = append(urls, map[string]string{
						"id":  id,
						"url": fmt.Sprintf("https://mail.google.com/mail/?authuser=%s#all/%s", url.QueryEscape(account), id),
					})
				}
				return outfmt.WriteJSON(os.Stdout, map[string]any{"urls": urls})
			}
			for _, id := range args {
				url := fmt.Sprintf("https://mail.google.com/mail/?authuser=%s#all/%s", url.QueryEscape(account), id)
				u.Out().Printf("%s\t%s", id, url)
			}
			return nil
		},
	}
}

type attachmentInfo struct {
	Filename     string
	Size         int64
	MimeType     string
	AttachmentID string
}

func collectAttachments(p *gmail.MessagePart) []attachmentInfo {
	if p == nil {
		return nil
	}
	var out []attachmentInfo
	if p.Filename != "" && p.Body != nil && p.Body.AttachmentId != "" {
		out = append(out, attachmentInfo{
			Filename:     p.Filename,
			Size:         p.Body.Size,
			MimeType:     p.MimeType,
			AttachmentID: p.Body.AttachmentId,
		})
	}
	for _, part := range p.Parts {
		out = append(out, collectAttachments(part)...)
	}
	return out
}

func bestBodyText(p *gmail.MessagePart) string {
	if p == nil {
		return ""
	}
	plain := findPartBody(p, "text/plain")
	if plain != "" {
		return plain
	}
	html := findPartBody(p, "text/html")
	return html
}

func findPartBody(p *gmail.MessagePart, mimeType string) string {
	if p == nil {
		return ""
	}
	if p.MimeType == mimeType && p.Body != nil && p.Body.Data != "" {
		s, err := decodeBase64URL(p.Body.Data)
		if err == nil {
			return s
		}
	}
	for _, part := range p.Parts {
		if s := findPartBody(part, mimeType); s != "" {
			return s
		}
	}
	return ""
}

func decodeBase64URL(s string) (string, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func downloadAttachment(cmd *cobra.Command, svc *gmail.Service, messageID string, a attachmentInfo, dir string) (string, bool, error) {
	if strings.TrimSpace(messageID) == "" || strings.TrimSpace(a.AttachmentID) == "" {
		return "", false, errors.New("missing messageID/attachmentID")
	}
	shortID := a.AttachmentID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	// Sanitize filename to prevent path traversal attacks
	safeFilename := filepath.Base(a.Filename)
	if safeFilename == "" || safeFilename == "." || safeFilename == ".." {
		safeFilename = "attachment"
	}
	filename := fmt.Sprintf("%s_%s_%s", messageID, shortID, safeFilename)
	outPath := filepath.Join(dir, filename)

	if st, err := os.Stat(outPath); err == nil && st.Size() == a.Size && a.Size > 0 {
		return outPath, true, nil
	}

	body, err := svc.Users.Messages.Attachments.Get("me", messageID, a.AttachmentID).Context(cmd.Context()).Do()
	if err != nil {
		return "", false, err
	}
	if body == nil || body.Data == "" {
		return "", false, errors.New("empty attachment data")
	}
	data, err := base64.RawURLEncoding.DecodeString(body.Data)
	if err != nil {
		return "", false, err
	}
	if err := os.WriteFile(outPath, data, 0o600); err != nil {
		return "", false, err
	}
	return outPath, false, nil
}
