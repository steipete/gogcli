package cmd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

func TestExecute_GmailAttachment_OutPath_JSON(t *testing.T) {
	origNew := newGmailService
	t.Cleanup(func() { newGmailService = origNew })

	var attachmentCalls int32
	attachmentData := []byte("abc")
	attachmentEncoded := base64.RawURLEncoding.EncodeToString(attachmentData)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/gmail/v1/users/me/messages/m1/attachments/a1"):
			atomic.AddInt32(&attachmentCalls, 1)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"data": attachmentEncoded})
			return
		case strings.Contains(r.URL.Path, "/gmail/v1/users/me/messages/m1"):
			if got := r.URL.Query().Get("format"); got != "full" {
				t.Fatalf("format=%q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "m1",
				"payload": map[string]any{
					"parts": []map[string]any{
						{
							"filename": "file.txt",
							"mimeType": "text/plain",
							"body": map[string]any{
								"attachmentId": "a1",
								"size":         len(attachmentData),
							},
						},
					},
				},
			})
			return
		default:
			http.NotFound(w, r)
			return
		}
	}))
	defer srv.Close()

	svc, err := gmail.NewService(context.Background(),
		option.WithoutAuthentication(),
		option.WithHTTPClient(srv.Client()),
		option.WithEndpoint(srv.URL+"/"),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	newGmailService = func(context.Context, string) (*gmail.Service, error) { return svc, nil }

	outPath := filepath.Join(t.TempDir(), "a.bin")

	run := func() (string, map[string]any) {
		out := captureStdout(t, func() {
			_ = captureStderr(t, func() {
				if execErr := Execute([]string{
					"--output", "json",
					"--account", "a@b.com",
					"gmail", "attachment", "m1", "a1",
					"--out", outPath,
				}); execErr != nil {
					t.Fatalf("Execute: %v", execErr)
				}
			})
		})
		var parsed map[string]any
		if unmarshalErr := json.Unmarshal([]byte(out), &parsed); unmarshalErr != nil {
			t.Fatalf("json parse: %v\nout=%q", unmarshalErr, out)
		}
		return out, parsed
	}

	_, parsed1 := run()
	if atomic.LoadInt32(&attachmentCalls) != 1 {
		t.Fatalf("attachmentCalls=%d", attachmentCalls)
	}
	if parsed1["path"] != outPath {
		t.Fatalf("path=%v", parsed1["path"])
	}
	if parsed1["cached"] != false {
		t.Fatalf("cached=%v", parsed1["cached"])
	}

	b, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(b) != string(attachmentData) {
		t.Fatalf("content=%q", string(b))
	}

	_, parsed2 := run()
	if atomic.LoadInt32(&attachmentCalls) != 1 {
		t.Fatalf("attachmentCalls=%d", attachmentCalls)
	}
	if parsed2["cached"] != true {
		t.Fatalf("cached=%v", parsed2["cached"])
	}
}

func TestExecute_GmailAttachment_NotFound(t *testing.T) {
	origNew := newGmailService
	t.Cleanup(func() { newGmailService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/gmail/v1/users/me/messages/m1") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "m1",
			"payload": map[string]any{
				"parts": []map[string]any{
					{
						"filename": "file.txt",
						"mimeType": "text/plain",
						"body": map[string]any{
							"attachmentId": "other",
							"size":         3,
						},
					},
				},
			},
		})
	}))
	defer srv.Close()

	svc, err := gmail.NewService(context.Background(),
		option.WithoutAuthentication(),
		option.WithHTTPClient(srv.Client()),
		option.WithEndpoint(srv.URL+"/"),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	newGmailService = func(context.Context, string) (*gmail.Service, error) { return svc, nil }

	outPath := filepath.Join(t.TempDir(), "a.bin")

	err = Execute([]string{
		"--output", "json",
		"--account", "a@b.com",
		"gmail", "attachment", "m1", "a1",
		"--out", outPath,
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "attachment not found") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, statErr := os.Stat(outPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected no file written, stat=%v", statErr)
	}
}
