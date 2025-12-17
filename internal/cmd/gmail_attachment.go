package cmd

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steipete/gogcli/internal/config"
	"github.com/steipete/gogcli/internal/outfmt"
	"github.com/steipete/gogcli/internal/ui"
	"google.golang.org/api/gmail/v1"
)

func newGmailAttachmentCmd(flags *rootFlags) *cobra.Command {
	var outPath string
	var filename string

	cmd := &cobra.Command{
		Use:   "attachment <messageId> <attachmentId>",
		Short: "Download a single attachment",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			messageID := strings.TrimSpace(args[0])
			attachmentID := strings.TrimSpace(args[1])
			if messageID == "" || attachmentID == "" {
				return errors.New("messageId/attachmentId required")
			}

			svc, err := newGmailService(cmd.Context(), account)
			if err != nil {
				return err
			}

			info, err := findAttachmentInfo(cmd, svc, messageID, attachmentID, filename)
			if err != nil {
				return err
			}

			if strings.TrimSpace(outPath) == "" {
				dir, dirErr := config.EnsureGmailAttachmentsDir()
				if dirErr != nil {
					return dirErr
				}
				path, cached, dlErr := downloadAttachment(cmd, svc, messageID, info, dir)
				if dlErr != nil {
					return dlErr
				}
				if outfmt.IsJSON(cmd.Context()) {
					return outfmt.WriteJSON(os.Stdout, map[string]any{"path": path, "cached": cached})
				}
				u.Out().Printf("path\t%s", path)
				u.Out().Printf("cached\t%t", cached)
				return nil
			}

			expectedSize := info.Size
			if expectedSize <= 0 {
				expectedSize = -1
			}
			path, cached, bytes, err := downloadAttachmentToPath(cmd, svc, messageID, info.AttachmentID, outPath, expectedSize)
			if err != nil {
				return err
			}
			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"path": path, "cached": cached, "bytes": bytes})
			}
			u.Out().Printf("path\t%s", path)
			u.Out().Printf("cached\t%t", cached)
			u.Out().Printf("bytes\t%d", bytes)
			return nil
		},
	}

	cmd.Flags().StringVar(&outPath, "out", "", "Write to a specific path (default: gogcli config dir)")
	cmd.Flags().StringVar(&filename, "filename", "", "Override inferred filename (only used when --out is empty)")
	return cmd
}

func findAttachmentInfo(cmd *cobra.Command, svc *gmail.Service, messageID string, attachmentID string, filename string) (attachmentInfo, error) {
	msg, err := svc.Users.Messages.Get("me", messageID).Format("full").Context(cmd.Context()).Do()
	if err != nil {
		return attachmentInfo{}, err
	}
	if msg == nil {
		return attachmentInfo{}, errors.New("message not found")
	}
	for _, a := range collectAttachments(msg.Payload) {
		if a.AttachmentID == attachmentID {
			if strings.TrimSpace(filename) != "" {
				a.Filename = filename
			}
			if strings.TrimSpace(a.Filename) == "" {
				a.Filename = "attachment.bin"
			}
			return a, nil
		}
	}
	return attachmentInfo{}, errors.New("attachment not found in message payload (need full payload to infer filename)")
}

func downloadAttachmentToPath(
	cmd *cobra.Command,
	svc *gmail.Service,
	messageID string,
	attachmentID string,
	outPath string,
	expectedSize int64,
) (string, bool, int64, error) {
	if strings.TrimSpace(outPath) == "" {
		return "", false, 0, errors.New("missing outPath")
	}

	if expectedSize > 0 {
		if st, err := os.Stat(outPath); err == nil && st.Size() == expectedSize {
			return outPath, true, st.Size(), nil
		}
	}

	body, err := svc.Users.Messages.Attachments.Get("me", messageID, attachmentID).Context(cmd.Context()).Do()
	if err != nil {
		return "", false, 0, err
	}
	if body == nil || body.Data == "" {
		return "", false, 0, errors.New("empty attachment data")
	}
	data, err := base64.RawURLEncoding.DecodeString(body.Data)
	if err != nil {
		return "", false, 0, err
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return "", false, 0, err
	}
	if err := os.WriteFile(outPath, data, 0o600); err != nil {
		return "", false, 0, err
	}
	return outPath, false, int64(len(data)), nil
}
