package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/steipete/gogcli/internal/config"
	"github.com/steipete/gogcli/internal/googleapi"
	"github.com/steipete/gogcli/internal/outfmt"
	"github.com/steipete/gogcli/internal/ui"
	"google.golang.org/api/drive/v3"
	gapi "google.golang.org/api/googleapi"
)

var newDriveService = googleapi.NewDrive

func newDriveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "drive",
		Short: "Google Drive",
	}

	cmd.AddCommand(newDriveLsCmd(flags))
	cmd.AddCommand(newDriveSearchCmd(flags))
	cmd.AddCommand(newDriveGetCmd(flags))
	cmd.AddCommand(newDriveDownloadCmd(flags))
	cmd.AddCommand(newDriveUploadCmd(flags))
	cmd.AddCommand(newDriveMkdirCmd(flags))
	cmd.AddCommand(newDriveDeleteCmd(flags))
	cmd.AddCommand(newDriveMoveCmd(flags))
	cmd.AddCommand(newDriveRenameCmd(flags))
	cmd.AddCommand(newDriveShareCmd(flags))
	cmd.AddCommand(newDriveUnshareCmd(flags))
	cmd.AddCommand(newDrivePermissionsCmd(flags))
	cmd.AddCommand(newDriveURLCmd(flags))

	return cmd
}

func newDriveLsCmd(flags *rootFlags) *cobra.Command {
	var max int64
	var page string
	var query string

	cmd := &cobra.Command{
		Use:   "ls [folderId]",
		Short: "List files in a folder (default: root)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}

			folderID := "root"
			if len(args) == 1 {
				folderID = args[0]
			}

			svc, err := newDriveService(cmd.Context(), account)
			if err != nil {
				return err
			}

			q := buildDriveListQuery(folderID, query)

			resp, err := svc.Files.List().
				Q(q).
				PageSize(max).
				PageToken(page).
				OrderBy("modifiedTime desc").
				SupportsAllDrives(true).
				IncludeItemsFromAllDrives(true).
				Fields("nextPageToken, files(id, name, mimeType, size, modifiedTime, parents, webViewLink)").
				Context(cmd.Context()).
				Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{
					"files":         resp.Files,
					"nextPageToken": resp.NextPageToken,
				})
			}

			if len(resp.Files) == 0 {
				u.Err().Println("No files")
				return nil
			}

			tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tNAME\tTYPE\tSIZE\tMODIFIED")
			for _, f := range resp.Files {
				fmt.Fprintf(
					tw,
					"%s\t%s\t%s\t%s\t%s\n",
					f.Id,
					f.Name,
					driveType(f.MimeType),
					formatDriveSize(f.Size),
					formatDateTime(f.ModifiedTime),
				)
			}
			_ = tw.Flush()

			if resp.NextPageToken != "" {
				u.Err().Printf("# Next page: --page %s", resp.NextPageToken)
			}
			return nil
		},
	}

	cmd.Flags().Int64Var(&max, "max", 20, "Max results")
	cmd.Flags().StringVar(&page, "page", "", "Page token")
	cmd.Flags().StringVar(&query, "query", "", "Drive query filter")
	return cmd
}

func newDriveSearchCmd(flags *rootFlags) *cobra.Command {
	var max int64
	var page string

	cmd := &cobra.Command{
		Use:   "search <text>",
		Short: "Full-text search across Drive",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			text := strings.Join(args, " ")

			svc, err := newDriveService(cmd.Context(), account)
			if err != nil {
				return err
			}

			resp, err := svc.Files.List().
				Q(buildDriveSearchQuery(text)).
				PageSize(max).
				PageToken(page).
				OrderBy("modifiedTime desc").
				SupportsAllDrives(true).
				IncludeItemsFromAllDrives(true).
				Fields("nextPageToken, files(id, name, mimeType, size, modifiedTime, parents, webViewLink)").
				Context(cmd.Context()).
				Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{
					"files":         resp.Files,
					"nextPageToken": resp.NextPageToken,
				})
			}

			if len(resp.Files) == 0 {
				u.Err().Println("No results")
				return nil
			}

			tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tNAME\tTYPE\tSIZE\tMODIFIED")
			for _, f := range resp.Files {
				fmt.Fprintf(
					tw,
					"%s\t%s\t%s\t%s\t%s\n",
					f.Id,
					f.Name,
					driveType(f.MimeType),
					formatDriveSize(f.Size),
					formatDateTime(f.ModifiedTime),
				)
			}
			_ = tw.Flush()

			if resp.NextPageToken != "" {
				u.Err().Printf("# Next page: --page %s", resp.NextPageToken)
			}
			return nil
		},
	}

	cmd.Flags().Int64Var(&max, "max", 20, "Max results")
	cmd.Flags().StringVar(&page, "page", "", "Page token")
	return cmd
}

func newDriveGetCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get <fileId>",
		Short: "Get file metadata",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			fileID := args[0]

			svc, err := newDriveService(cmd.Context(), account)
			if err != nil {
				return err
			}

			f, err := svc.Files.Get(fileID).
				SupportsAllDrives(true).
				Fields("id, name, mimeType, size, modifiedTime, createdTime, parents, webViewLink, description, starred").
				Context(cmd.Context()).
				Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"file": f})
			}

			u.Out().Printf("id\t%s", f.Id)
			u.Out().Printf("name\t%s", f.Name)
			u.Out().Printf("type\t%s", f.MimeType)
			u.Out().Printf("size\t%s", formatDriveSize(f.Size))
			u.Out().Printf("created\t%s", f.CreatedTime)
			u.Out().Printf("modified\t%s", f.ModifiedTime)
			if f.Description != "" {
				u.Out().Printf("description\t%s", f.Description)
			}
			u.Out().Printf("starred\t%t", f.Starred)
			if f.WebViewLink != "" {
				u.Out().Printf("link\t%s", f.WebViewLink)
			}
			return nil
		},
	}
}

func newDriveDownloadCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "download <fileId> [destPath]",
		Short: "Download a file (Google Docs exported)",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}

			fileID := args[0]
			destPath := ""
			if len(args) == 2 {
				destPath = args[1]
			}

			svc, err := newDriveService(cmd.Context(), account)
			if err != nil {
				return err
			}

			meta, err := svc.Files.Get(fileID).
				SupportsAllDrives(true).
				Fields("id, name, mimeType").
				Context(cmd.Context()).
				Do()
			if err != nil {
				return err
			}
			if meta.Name == "" {
				return errors.New("file has no name")
			}

			if destPath == "" {
				dir, dirErr := config.EnsureDriveDownloadsDir()
				if dirErr != nil {
					return dirErr
				}
				// Sanitize filename to prevent path traversal
				safeName := filepath.Base(meta.Name)
				if safeName == "" || safeName == "." || safeName == ".." {
					safeName = "download"
				}
				destPath = filepath.Join(dir, fmt.Sprintf("%s_%s", fileID, safeName))
			}

			outPath, size, err := downloadDriveFile(cmd.Context(), svc, meta, destPath)
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{
					"path": outPath,
					"size": size,
				})
			}

			u.Out().Printf("path\t%s", outPath)
			u.Out().Printf("size\t%s", formatDriveSize(size))
			return nil
		},
	}
}

func newDriveUploadCmd(flags *rootFlags) *cobra.Command {
	var name string
	var folderID string

	cmd := &cobra.Command{
		Use:   "upload <localPath>",
		Short: "Upload a file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}

			localPath := args[0]
			f, err := os.Open(localPath)
			if err != nil {
				return err
			}
			defer f.Close()

			fileName := name
			if fileName == "" {
				fileName = filepath.Base(localPath)
			}

			svc, err := newDriveService(cmd.Context(), account)
			if err != nil {
				return err
			}

			meta := &drive.File{Name: fileName}
			if folderID != "" {
				meta.Parents = []string{folderID}
			}

			mimeType := guessMimeType(localPath)
			created, err := svc.Files.Create(meta).
				SupportsAllDrives(true).
				Media(f, gapi.ContentType(mimeType)).
				Fields("id, name, mimeType, size, webViewLink").
				Context(cmd.Context()).
				Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"file": created})
			}

			u.Out().Printf("id\t%s", created.Id)
			u.Out().Printf("name\t%s", created.Name)
			if created.WebViewLink != "" {
				u.Out().Printf("link\t%s", created.WebViewLink)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Override filename")
	cmd.Flags().StringVar(&folderID, "folder", "", "Destination folder ID")
	return cmd
}

func newDriveMkdirCmd(flags *rootFlags) *cobra.Command {
	var parent string

	cmd := &cobra.Command{
		Use:   "mkdir <name>",
		Short: "Create a folder",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}

			name := args[0]

			svc, err := newDriveService(cmd.Context(), account)
			if err != nil {
				return err
			}

			f := &drive.File{
				Name:     name,
				MimeType: "application/vnd.google-apps.folder",
			}
			if parent != "" {
				f.Parents = []string{parent}
			}

			created, err := svc.Files.Create(f).
				SupportsAllDrives(true).
				Fields("id, name, webViewLink").
				Context(cmd.Context()).
				Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"folder": created})
			}

			u.Out().Printf("id\t%s", created.Id)
			u.Out().Printf("name\t%s", created.Name)
			if created.WebViewLink != "" {
				u.Out().Printf("link\t%s", created.WebViewLink)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&parent, "parent", "", "Parent folder ID")
	return cmd
}

func newDriveDeleteCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <fileId>",
		Short: "Delete a file (moves to trash)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			fileID := args[0]

			svc, err := newDriveService(cmd.Context(), account)
			if err != nil {
				return err
			}

			if err := svc.Files.Delete(fileID).SupportsAllDrives(true).Context(cmd.Context()).Do(); err != nil {
				return err
			}
			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{
					"deleted": true,
					"id":      fileID,
				})
			}
			u.Out().Printf("deleted\ttrue")
			u.Out().Printf("id\t%s", fileID)
			return nil
		},
	}
}

func newDriveMoveCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "move <fileId> <newParentId>",
		Short: "Move a file to a different folder",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			fileID := args[0]
			newParentID := args[1]

			svc, err := newDriveService(cmd.Context(), account)
			if err != nil {
				return err
			}

			meta, err := svc.Files.Get(fileID).
				SupportsAllDrives(true).
				Fields("id, name, parents").
				Context(cmd.Context()).
				Do()
			if err != nil {
				return err
			}

			call := svc.Files.Update(fileID, &drive.File{}).
				SupportsAllDrives(true).
				AddParents(newParentID).
				Fields("id, name, parents, webViewLink")
			if len(meta.Parents) > 0 {
				call = call.RemoveParents(strings.Join(meta.Parents, ","))
			}

			updated, err := call.Context(cmd.Context()).Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"file": updated})
			}

			u.Out().Printf("id\t%s", updated.Id)
			u.Out().Printf("name\t%s", updated.Name)
			return nil
		},
	}
}

func newDriveRenameCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "rename <fileId> <newName>",
		Short: "Rename a file or folder",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			fileID := args[0]
			newName := args[1]

			svc, err := newDriveService(cmd.Context(), account)
			if err != nil {
				return err
			}

			updated, err := svc.Files.Update(fileID, &drive.File{Name: newName}).
				SupportsAllDrives(true).
				Fields("id, name").
				Context(cmd.Context()).
				Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"file": updated})
			}

			u.Out().Printf("id\t%s", updated.Id)
			u.Out().Printf("name\t%s", updated.Name)
			return nil
		},
	}
}

func newDriveShareCmd(flags *rootFlags) *cobra.Command {
	var anyone bool
	var email string
	var role string
	var discoverable bool

	cmd := &cobra.Command{
		Use:   "share <fileId>",
		Short: "Share a file or folder",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			fileID := args[0]

			if !anyone && email == "" {
				return errors.New("must specify --anyone or --email")
			}
			if role == "" {
				role = "reader"
			}
			if role != "reader" && role != "writer" {
				return errors.New("invalid --role (expected reader|writer)")
			}

			svc, err := newDriveService(cmd.Context(), account)
			if err != nil {
				return err
			}

			perm := &drive.Permission{Role: role}
			if anyone {
				perm.Type = "anyone"
				perm.AllowFileDiscovery = discoverable
			} else {
				perm.Type = "user"
				perm.EmailAddress = email
			}

			created, err := svc.Permissions.Create(fileID, perm).
				SupportsAllDrives(true).
				SendNotificationEmail(false).
				Fields("id, type, role, emailAddress").
				Context(cmd.Context()).
				Do()
			if err != nil {
				return err
			}

			link, err := driveWebLink(cmd.Context(), svc, fileID)
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{
					"link":         link,
					"permissionId": created.Id,
					"permission":   created,
				})
			}

			u.Out().Printf("link\t%s", link)
			u.Out().Printf("permission_id\t%s", created.Id)
			return nil
		},
	}

	cmd.Flags().BoolVar(&anyone, "anyone", false, "Make publicly accessible")
	cmd.Flags().StringVar(&email, "email", "", "Share with specific user")
	cmd.Flags().StringVar(&role, "role", "reader", "Permission: reader|writer")
	cmd.Flags().BoolVar(&discoverable, "discoverable", false, "Allow file discovery in search (anyone/domain only)")
	return cmd
}

func newDriveUnshareCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "unshare <fileId> <permissionId>",
		Short: "Remove a permission from a file",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			fileID := args[0]
			permissionID := args[1]

			svc, err := newDriveService(cmd.Context(), account)
			if err != nil {
				return err
			}

			if err := svc.Permissions.Delete(fileID, permissionID).SupportsAllDrives(true).Context(cmd.Context()).Do(); err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{
					"removed":      true,
					"fileId":       fileID,
					"permissionId": permissionID,
				})
			}

			u.Out().Printf("removed\ttrue")
			u.Out().Printf("file_id\t%s", fileID)
			u.Out().Printf("permission_id\t%s", permissionID)
			return nil
		},
	}
}

func newDrivePermissionsCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "permissions <fileId>",
		Short: "List permissions on a file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			fileID := args[0]

			svc, err := newDriveService(cmd.Context(), account)
			if err != nil {
				return err
			}

			resp, err := svc.Permissions.List(fileID).
				SupportsAllDrives(true).
				Fields("permissions(id, type, role, emailAddress)").
				Context(cmd.Context()).
				Do()
			if err != nil {
				return err
			}
			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"permissions": resp.Permissions})
			}
			if len(resp.Permissions) == 0 {
				u.Err().Println("No permissions")
				return nil
			}

			tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tTYPE\tROLE\tEMAIL")
			for _, p := range resp.Permissions {
				email := p.EmailAddress
				if email == "" {
					email = "-"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", p.Id, p.Type, p.Role, email)
			}
			_ = tw.Flush()
			return nil
		},
	}
}

func newDriveURLCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "url <fileIds...>",
		Short: "Print web URLs for files",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}

			svc, err := newDriveService(cmd.Context(), account)
			if err != nil {
				return err
			}

			for _, id := range args {
				link, err := driveWebLink(cmd.Context(), svc, id)
				if err != nil {
					return err
				}
				if outfmt.IsJSON(cmd.Context()) {
					// collected below
				} else {
					u.Out().Printf("%s\t%s", id, link)
				}
			}
			if outfmt.IsJSON(cmd.Context()) {
				urls := make([]map[string]string, 0, len(args))
				for _, id := range args {
					link, err := driveWebLink(cmd.Context(), svc, id)
					if err != nil {
						return err
					}
					urls = append(urls, map[string]string{"id": id, "url": link})
				}
				return outfmt.WriteJSON(os.Stdout, map[string]any{"urls": urls})
			}
			return nil
		},
	}
}

func buildDriveListQuery(folderID string, userQuery string) string {
	q := strings.TrimSpace(userQuery)
	parent := fmt.Sprintf("'%s' in parents", folderID)
	if q != "" {
		q = q + " and " + parent
	} else {
		q = parent
	}
	if !strings.Contains(q, "trashed") {
		q = q + " and trashed = false"
	}
	return q
}

func buildDriveSearchQuery(text string) string {
	q := fmt.Sprintf("fullText contains '%s'", escapeDriveQueryString(text))
	return q + " and trashed = false"
}

func escapeDriveQueryString(s string) string {
	// Escape backslashes first, then single quotes
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "'", "\\'")
	return s
}

func driveType(mimeType string) string {
	if mimeType == "application/vnd.google-apps.folder" {
		return "folder"
	}
	return "file"
}

func formatDateTime(iso string) string {
	if iso == "" {
		return "-"
	}
	if len(iso) >= 16 {
		return strings.ReplaceAll(iso[:16], "T", " ")
	}
	return iso
}

func formatDriveSize(bytes int64) string {
	if bytes <= 0 {
		return "-"
	}
	const unit = 1024.0
	b := float64(bytes)
	units := []string{"B", "KB", "MB", "GB", "TB"}
	i := 0
	for b >= unit && i < len(units)-1 {
		b /= unit
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d B", bytes)
	}
	return fmt.Sprintf("%.1f %s", b, units[i])
}

func guessMimeType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".pdf":
		return "application/pdf"
	case ".doc":
		return "application/msword"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xls":
		return "application/vnd.ms-excel"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".ppt":
		return "application/vnd.ms-powerpoint"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".txt":
		return "text/plain"
	case ".html":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".json":
		return "application/json"
	case ".zip":
		return "application/zip"
	case ".csv":
		return "text/csv"
	case ".md":
		return "text/markdown"
	default:
		return "application/octet-stream"
	}
}

func downloadDriveFile(ctx context.Context, svc *drive.Service, meta *drive.File, destPath string) (string, int64, error) {
	isGoogleDoc := strings.HasPrefix(meta.MimeType, "application/vnd.google-apps.")

	var (
		resp    *http.Response
		outPath string
		err     error
	)

	if isGoogleDoc {
		exportMimeType := driveExportMimeType(meta.MimeType)
		outPath = replaceExt(destPath, driveExportExtension(exportMimeType))
		resp, err = driveExportDownload(ctx, svc, meta.Id, exportMimeType)
	} else {
		outPath = destPath
		resp, err = driveDownload(ctx, svc, meta.Id)
	}
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", 0, fmt.Errorf("download failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	f, err := os.Create(outPath)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	n, err := io.Copy(f, resp.Body)
	if err != nil {
		return "", 0, err
	}
	return outPath, n, nil
}

var driveDownload = func(ctx context.Context, svc *drive.Service, fileID string) (*http.Response, error) {
	return svc.Files.Get(fileID).SupportsAllDrives(true).Context(ctx).Download()
}

var driveExportDownload = func(ctx context.Context, svc *drive.Service, fileID string, mimeType string) (*http.Response, error) {
	return svc.Files.Export(fileID, mimeType).Context(ctx).Download()
}

func replaceExt(path string, ext string) string {
	base := strings.TrimSuffix(path, filepath.Ext(path))
	return base + ext
}

func driveExportMimeType(googleMimeType string) string {
	switch googleMimeType {
	case "application/vnd.google-apps.document":
		return "application/pdf"
	case "application/vnd.google-apps.spreadsheet":
		return "text/csv"
	case "application/vnd.google-apps.presentation":
		return "application/pdf"
	case "application/vnd.google-apps.drawing":
		return "image/png"
	default:
		return "application/pdf"
	}
}

func driveExportExtension(mimeType string) string {
	switch mimeType {
	case "application/pdf":
		return ".pdf"
	case "text/csv":
		return ".csv"
	case "image/png":
		return ".png"
	case "text/plain":
		return ".txt"
	default:
		return ".pdf"
	}
}

func driveWebLink(ctx context.Context, svc *drive.Service, fileID string) (string, error) {
	f, err := svc.Files.Get(fileID).SupportsAllDrives(true).Fields("webViewLink").Context(ctx).Do()
	if err != nil {
		return "", err
	}
	if f.WebViewLink != "" {
		return f.WebViewLink, nil
	}
	return fmt.Sprintf("https://drive.google.com/file/d/%s/view", fileID), nil
}
