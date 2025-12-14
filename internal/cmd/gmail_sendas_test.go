package cmd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/steipete/gogcli/internal/outfmt"
	"github.com/steipete/gogcli/internal/ui"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

func TestGmailSendAsListCmd_JSON(t *testing.T) {
	origNew := newGmailService
	t.Cleanup(func() { newGmailService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/settings/sendAs") && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sendAs": []map[string]any{
					{
						"sendAsEmail":        "primary@example.com",
						"displayName":        "Primary User",
						"isDefault":          true,
						"isPrimary":          true,
						"treatAsAlias":       false,
						"verificationStatus": "accepted",
					},
					{
						"sendAsEmail":        "work@company.com",
						"displayName":        "Work Alias",
						"isDefault":          false,
						"isPrimary":          false,
						"treatAsAlias":       true,
						"verificationStatus": "accepted",
					},
				},
			})
			return
		}
		http.NotFound(w, r)
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

	flags := &rootFlags{Account: "a@b.com"}

	out := captureStdout(t, func() {
		u, uiErr := ui.New(ui.Options{Stdout: io.Discard, Stderr: io.Discard, Color: "never"})
		if uiErr != nil {
			t.Fatalf("ui.New: %v", uiErr)
		}
		ctx := ui.WithUI(context.Background(), u)
		ctx = outfmt.WithMode(ctx, outfmt.ModeJSON)

		cmd := newGmailSendAsListCmd(flags)
		cmd.SetContext(ctx)
		cmd.SetArgs([]string{})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	var parsed struct {
		SendAs []struct {
			SendAsEmail        string `json:"sendAsEmail"`
			DisplayName        string `json:"displayName"`
			IsDefault          bool   `json:"isDefault"`
			VerificationStatus string `json:"verificationStatus"`
		} `json:"sendAs"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json parse: %v\nout=%q", err, out)
	}
	if len(parsed.SendAs) != 2 {
		t.Fatalf("unexpected sendAs count: %d", len(parsed.SendAs))
	}
	if parsed.SendAs[0].SendAsEmail != "primary@example.com" {
		t.Fatalf("unexpected first sendAs: %#v", parsed.SendAs[0])
	}
	if parsed.SendAs[1].SendAsEmail != "work@company.com" {
		t.Fatalf("unexpected second sendAs: %#v", parsed.SendAs[1])
	}
}

func TestGmailSendAsGetCmd_JSON(t *testing.T) {
	origNew := newGmailService
	t.Cleanup(func() { newGmailService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/settings/sendAs/work@company.com") && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sendAsEmail":        "work@company.com",
				"displayName":        "Work Alias",
				"replyToAddress":     "replies@company.com",
				"signature":          "<b>Signature</b>",
				"isDefault":          false,
				"isPrimary":          false,
				"treatAsAlias":       true,
				"verificationStatus": "accepted",
			})
			return
		}
		http.NotFound(w, r)
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

	flags := &rootFlags{Account: "a@b.com"}

	out := captureStdout(t, func() {
		u, uiErr := ui.New(ui.Options{Stdout: io.Discard, Stderr: io.Discard, Color: "never"})
		if uiErr != nil {
			t.Fatalf("ui.New: %v", uiErr)
		}
		ctx := ui.WithUI(context.Background(), u)
		ctx = outfmt.WithMode(ctx, outfmt.ModeJSON)

		cmd := newGmailSendAsGetCmd(flags)
		cmd.SetContext(ctx)
		cmd.SetArgs([]string{"work@company.com"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	var parsed struct {
		SendAs struct {
			SendAsEmail        string `json:"sendAsEmail"`
			DisplayName        string `json:"displayName"`
			ReplyToAddress     string `json:"replyToAddress"`
			VerificationStatus string `json:"verificationStatus"`
		} `json:"sendAs"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json parse: %v\nout=%q", err, out)
	}
	if parsed.SendAs.SendAsEmail != "work@company.com" {
		t.Fatalf("unexpected sendAs: %#v", parsed.SendAs)
	}
	if parsed.SendAs.DisplayName != "Work Alias" {
		t.Fatalf("unexpected displayName: %q", parsed.SendAs.DisplayName)
	}
}

func TestGmailBatchDeleteCmd_JSON(t *testing.T) {
	origNew := newGmailService
	t.Cleanup(func() { newGmailService = origNew })

	var receivedIDs []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/messages/batchDelete") && r.Method == http.MethodPost {
			var body struct {
				IDs []string `json:"ids"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			receivedIDs = body.IDs
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.NotFound(w, r)
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

	flags := &rootFlags{Account: "a@b.com"}

	out := captureStdout(t, func() {
		u, uiErr := ui.New(ui.Options{Stdout: io.Discard, Stderr: io.Discard, Color: "never"})
		if uiErr != nil {
			t.Fatalf("ui.New: %v", uiErr)
		}
		ctx := ui.WithUI(context.Background(), u)
		ctx = outfmt.WithMode(ctx, outfmt.ModeJSON)

		cmd := newGmailBatchDeleteCmd(flags)
		cmd.SetContext(ctx)
		cmd.SetArgs([]string{"msg1", "msg2", "msg3"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	if len(receivedIDs) != 3 || receivedIDs[0] != "msg1" {
		t.Fatalf("unexpected IDs sent: %v", receivedIDs)
	}

	var parsed struct {
		Deleted []string `json:"deleted"`
		Count   int      `json:"count"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json parse: %v\nout=%q", err, out)
	}
	if parsed.Count != 3 {
		t.Fatalf("unexpected count: %d", parsed.Count)
	}
}

func TestGmailBatchModifyCmd_JSON(t *testing.T) {
	origNew := newGmailService
	t.Cleanup(func() { newGmailService = origNew })

	var receivedRequest struct {
		IDs            []string `json:"ids"`
		AddLabelIds    []string `json:"addLabelIds"`
		RemoveLabelIds []string `json:"removeLabelIds"`
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/users/me/labels"):
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"labels": []map[string]any{
					{"id": "INBOX", "name": "INBOX", "type": "system"},
					{"id": "SPAM", "name": "SPAM", "type": "system"},
				},
			})
			return
		case strings.Contains(r.URL.Path, "/messages/batchModify") && r.Method == http.MethodPost:
			_ = json.NewDecoder(r.Body).Decode(&receivedRequest)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.NotFound(w, r)
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

	flags := &rootFlags{Account: "a@b.com"}

	out := captureStdout(t, func() {
		u, uiErr := ui.New(ui.Options{Stdout: io.Discard, Stderr: io.Discard, Color: "never"})
		if uiErr != nil {
			t.Fatalf("ui.New: %v", uiErr)
		}
		ctx := ui.WithUI(context.Background(), u)
		ctx = outfmt.WithMode(ctx, outfmt.ModeJSON)

		cmd := newGmailBatchModifyCmd(flags)
		cmd.SetContext(ctx)
		cmd.SetArgs([]string{"msg1", "msg2"})
		_ = cmd.Flags().Set("add", "INBOX")
		_ = cmd.Flags().Set("remove", "SPAM")
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	if len(receivedRequest.IDs) != 2 {
		t.Fatalf("unexpected IDs: %v", receivedRequest.IDs)
	}
	if len(receivedRequest.AddLabelIds) != 1 || receivedRequest.AddLabelIds[0] != "INBOX" {
		t.Fatalf("unexpected addLabelIds: %v", receivedRequest.AddLabelIds)
	}
	if len(receivedRequest.RemoveLabelIds) != 1 || receivedRequest.RemoveLabelIds[0] != "SPAM" {
		t.Fatalf("unexpected removeLabelIds: %v", receivedRequest.RemoveLabelIds)
	}

	var parsed struct {
		Modified []string `json:"modified"`
		Count    int      `json:"count"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json parse: %v\nout=%q", err, out)
	}
	if parsed.Count != 2 {
		t.Fatalf("unexpected count: %d", parsed.Count)
	}
}

func TestGmailSendAsCreateCmd_JSON(t *testing.T) {
	origNew := newGmailService
	t.Cleanup(func() { newGmailService = origNew })

	var receivedBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/settings/sendAs") && r.Method == http.MethodPost {
			_ = json.NewDecoder(r.Body).Decode(&receivedBody)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sendAsEmail":        "alias@example.com",
				"displayName":        "Test Alias",
				"verificationStatus": "pending",
			})
			return
		}
		http.NotFound(w, r)
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

	flags := &rootFlags{Account: "a@b.com"}

	out := captureStdout(t, func() {
		u, uiErr := ui.New(ui.Options{Stdout: io.Discard, Stderr: io.Discard, Color: "never"})
		if uiErr != nil {
			t.Fatalf("ui.New: %v", uiErr)
		}
		ctx := ui.WithUI(context.Background(), u)
		ctx = outfmt.WithMode(ctx, outfmt.ModeJSON)

		cmd := newGmailSendAsCreateCmd(flags)
		cmd.SetContext(ctx)
		cmd.SetArgs([]string{"alias@example.com"})
		_ = cmd.Flags().Set("display-name", "Test Alias")
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	var parsed struct {
		SendAs struct {
			SendAsEmail        string `json:"sendAsEmail"`
			VerificationStatus string `json:"verificationStatus"`
		} `json:"sendAs"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json parse: %v\nout=%q", err, out)
	}
	if parsed.SendAs.SendAsEmail != "alias@example.com" {
		t.Fatalf("unexpected sendAs: %#v", parsed.SendAs)
	}
	if parsed.SendAs.VerificationStatus != "pending" {
		t.Fatalf("unexpected status: %q", parsed.SendAs.VerificationStatus)
	}
}

func TestGmailSendAsDeleteCmd_JSON(t *testing.T) {
	origNew := newGmailService
	t.Cleanup(func() { newGmailService = origNew })

	var deletedEmail string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/settings/sendAs/") && r.Method == http.MethodDelete {
			parts := strings.Split(r.URL.Path, "/")
			deletedEmail = parts[len(parts)-1]
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.NotFound(w, r)
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

	flags := &rootFlags{Account: "a@b.com"}

	out := captureStdout(t, func() {
		u, uiErr := ui.New(ui.Options{Stdout: io.Discard, Stderr: io.Discard, Color: "never"})
		if uiErr != nil {
			t.Fatalf("ui.New: %v", uiErr)
		}
		ctx := ui.WithUI(context.Background(), u)
		ctx = outfmt.WithMode(ctx, outfmt.ModeJSON)

		cmd := newGmailSendAsDeleteCmd(flags)
		cmd.SetContext(ctx)
		cmd.SetArgs([]string{"delete-me@example.com"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	if deletedEmail != "delete-me@example.com" {
		t.Fatalf("unexpected deleted email: %q", deletedEmail)
	}

	var parsed struct {
		Email   string `json:"email"`
		Deleted bool   `json:"deleted"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json parse: %v\nout=%q", err, out)
	}
	if !parsed.Deleted {
		t.Fatalf("expected deleted=true")
	}
}

func TestGmailSendAsVerifyCmd_JSON(t *testing.T) {
	origNew := newGmailService
	t.Cleanup(func() { newGmailService = origNew })

	var verifiedEmail string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/settings/sendAs/") && strings.HasSuffix(r.URL.Path, "/verify") && r.Method == http.MethodPost {
			parts := strings.Split(r.URL.Path, "/")
			verifiedEmail = parts[len(parts)-2]
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.NotFound(w, r)
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

	flags := &rootFlags{Account: "a@b.com"}

	out := captureStdout(t, func() {
		u, uiErr := ui.New(ui.Options{Stdout: io.Discard, Stderr: io.Discard, Color: "never"})
		if uiErr != nil {
			t.Fatalf("ui.New: %v", uiErr)
		}
		ctx := ui.WithUI(context.Background(), u)
		ctx = outfmt.WithMode(ctx, outfmt.ModeJSON)

		cmd := newGmailSendAsVerifyCmd(flags)
		cmd.SetContext(ctx)
		cmd.SetArgs([]string{"verify-me@example.com"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	if verifiedEmail != "verify-me@example.com" {
		t.Fatalf("unexpected verified email: %q", verifiedEmail)
	}

	var parsed struct {
		Email   string `json:"email"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json parse: %v\nout=%q", err, out)
	}
	if parsed.Email != "verify-me@example.com" {
		t.Fatalf("unexpected email: %q", parsed.Email)
	}
}

func TestGmailSendAsUpdateCmd_JSON(t *testing.T) {
	origNew := newGmailService
	t.Cleanup(func() { newGmailService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/settings/sendAs/update@example.com") && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sendAsEmail":        "update@example.com",
				"displayName":        "Old Name",
				"verificationStatus": "accepted",
			})
			return
		case strings.Contains(r.URL.Path, "/settings/sendAs/update@example.com") && r.Method == http.MethodPut:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sendAsEmail":        "update@example.com",
				"displayName":        "New Name",
				"verificationStatus": "accepted",
			})
			return
		}
		http.NotFound(w, r)
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

	flags := &rootFlags{Account: "a@b.com"}

	out := captureStdout(t, func() {
		u, uiErr := ui.New(ui.Options{Stdout: io.Discard, Stderr: io.Discard, Color: "never"})
		if uiErr != nil {
			t.Fatalf("ui.New: %v", uiErr)
		}
		ctx := ui.WithUI(context.Background(), u)
		ctx = outfmt.WithMode(ctx, outfmt.ModeJSON)

		cmd := newGmailSendAsUpdateCmd(flags)
		cmd.SetContext(ctx)
		cmd.SetArgs([]string{"update@example.com"})
		_ = cmd.Flags().Set("display-name", "New Name")
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	var parsed struct {
		SendAs struct {
			SendAsEmail string `json:"sendAsEmail"`
			DisplayName string `json:"displayName"`
		} `json:"sendAs"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json parse: %v\nout=%q", err, out)
	}
	if parsed.SendAs.DisplayName != "New Name" {
		t.Fatalf("unexpected displayName: %q", parsed.SendAs.DisplayName)
	}
}

func TestGmailSendFromFlag_ValidAlias(t *testing.T) {
	origNew := newGmailService
	t.Cleanup(func() { newGmailService = origNew })

	var sentMessage map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/settings/sendAs/alias@example.com") && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sendAsEmail":        "alias@example.com",
				"displayName":        "Work Alias",
				"verificationStatus": "accepted",
			})
			return
		case strings.Contains(r.URL.Path, "/messages/send") && r.Method == http.MethodPost:
			_ = json.NewDecoder(r.Body).Decode(&sentMessage)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":       "sent123",
				"threadId": "thread123",
			})
			return
		}
		http.NotFound(w, r)
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

	flags := &rootFlags{Account: "a@b.com"}

	out := captureStdout(t, func() {
		u, uiErr := ui.New(ui.Options{Stdout: io.Discard, Stderr: io.Discard, Color: "never"})
		if uiErr != nil {
			t.Fatalf("ui.New: %v", uiErr)
		}
		ctx := ui.WithUI(context.Background(), u)
		ctx = outfmt.WithMode(ctx, outfmt.ModeJSON)

		cmd := newGmailSendCmd(flags)
		cmd.SetContext(ctx)
		cmd.SetArgs([]string{})
		_ = cmd.Flags().Set("to", "recipient@example.com")
		_ = cmd.Flags().Set("subject", "Test")
		_ = cmd.Flags().Set("body", "Test body")
		_ = cmd.Flags().Set("from", "alias@example.com")
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	var parsed struct {
		MessageID string `json:"messageId"`
		From      string `json:"from"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json parse: %v\nout=%q", err, out)
	}
	if parsed.MessageID != "sent123" {
		t.Fatalf("unexpected messageId: %q", parsed.MessageID)
	}
	// Verify from includes display name
	if parsed.From != "Work Alias <alias@example.com>" {
		t.Fatalf("unexpected from: %q", parsed.From)
	}
}

func TestGmailSendFromFlag_NotVerified(t *testing.T) {
	origNew := newGmailService
	t.Cleanup(func() { newGmailService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/settings/sendAs/unverified@example.com") && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sendAsEmail":        "unverified@example.com",
				"verificationStatus": "pending",
			})
			return
		}
		http.NotFound(w, r)
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

	flags := &rootFlags{Account: "a@b.com"}

	u, _ := ui.New(ui.Options{Stdout: io.Discard, Stderr: io.Discard, Color: "never"})
	ctx := ui.WithUI(context.Background(), u)
	ctx = outfmt.WithMode(ctx, outfmt.ModeJSON)

	cmd := newGmailSendCmd(flags)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{})
	_ = cmd.Flags().Set("to", "recipient@example.com")
	_ = cmd.Flags().Set("subject", "Test")
	_ = cmd.Flags().Set("body", "Test body")
	_ = cmd.Flags().Set("from", "unverified@example.com")

	err = cmd.Execute()
	if err == nil {
		t.Fatalf("expected error for unverified alias")
	}
	if !strings.Contains(err.Error(), "not verified") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGmailSendFromFlag_NotExist(t *testing.T) {
	origNew := newGmailService
	t.Cleanup(func() { newGmailService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/settings/sendAs/") && r.Method == http.MethodGet {
			http.NotFound(w, r)
			return
		}
		http.NotFound(w, r)
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

	flags := &rootFlags{Account: "a@b.com"}

	u, _ := ui.New(ui.Options{Stdout: io.Discard, Stderr: io.Discard, Color: "never"})
	ctx := ui.WithUI(context.Background(), u)
	ctx = outfmt.WithMode(ctx, outfmt.ModeJSON)

	cmd := newGmailSendCmd(flags)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{})
	_ = cmd.Flags().Set("to", "recipient@example.com")
	_ = cmd.Flags().Set("subject", "Test")
	_ = cmd.Flags().Set("body", "Test body")
	_ = cmd.Flags().Set("from", "nonexistent@example.com")

	err = cmd.Execute()
	if err == nil {
		t.Fatalf("expected error for nonexistent alias")
	}
	if !strings.Contains(err.Error(), "invalid --from address") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGmailSendAsListCmd_TableOutput(t *testing.T) {
	origNew := newGmailService
	t.Cleanup(func() { newGmailService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/settings/sendAs") && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sendAs": []map[string]any{
					{
						"sendAsEmail":        "primary@example.com",
						"displayName":        "Primary User",
						"isDefault":          true,
						"treatAsAlias":       false,
						"verificationStatus": "accepted",
					},
					{
						"sendAsEmail":        "work@company.com",
						"displayName":        "Work Alias",
						"isDefault":          false,
						"treatAsAlias":       true,
						"verificationStatus": "pending",
					},
				},
			})
			return
		}
		http.NotFound(w, r)
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

	flags := &rootFlags{Account: "a@b.com"}

	out := captureStdout(t, func() {
		u, uiErr := ui.New(ui.Options{Stdout: io.Discard, Stderr: io.Discard, Color: "never"})
		if uiErr != nil {
			t.Fatalf("ui.New: %v", uiErr)
		}
		ctx := ui.WithUI(context.Background(), u)
		// Not setting JSON mode - should output table format

		cmd := newGmailSendAsListCmd(flags)
		cmd.SetContext(ctx)
		cmd.SetArgs([]string{})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	// Table output should contain headers and data
	if !strings.Contains(out, "EMAIL") {
		t.Fatalf("expected table header EMAIL, got: %q", out)
	}
	if !strings.Contains(out, "DISPLAY NAME") {
		t.Fatalf("expected table header DISPLAY NAME, got: %q", out)
	}
	if !strings.Contains(out, "primary@example.com") {
		t.Fatalf("expected primary@example.com in output, got: %q", out)
	}
	if !strings.Contains(out, "work@company.com") {
		t.Fatalf("expected work@company.com in output, got: %q", out)
	}
}

func TestGmailSendAsListCmd_EmptyList(t *testing.T) {
	origNew := newGmailService
	t.Cleanup(func() { newGmailService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/settings/sendAs") && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sendAs": []map[string]any{},
			})
			return
		}
		http.NotFound(w, r)
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

	flags := &rootFlags{Account: "a@b.com"}

	var stderrBuf strings.Builder
	u, uiErr := ui.New(ui.Options{Stdout: io.Discard, Stderr: &stderrBuf, Color: "never"})
	if uiErr != nil {
		t.Fatalf("ui.New: %v", uiErr)
	}
	ctx := ui.WithUI(context.Background(), u)
	// Not setting JSON mode - should output "No send-as aliases"

	cmd := newGmailSendAsListCmd(flags)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if !strings.Contains(stderrBuf.String(), "No send-as aliases") {
		t.Fatalf("expected 'No send-as aliases' in stderr, got: %q", stderrBuf.String())
	}
}

func TestGmailBatchModifyCmd_NoLabelsSpecified(t *testing.T) {
	origNew := newGmailService
	t.Cleanup(func() { newGmailService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
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

	flags := &rootFlags{Account: "a@b.com"}

	u, _ := ui.New(ui.Options{Stdout: io.Discard, Stderr: io.Discard, Color: "never"})
	ctx := ui.WithUI(context.Background(), u)
	ctx = outfmt.WithMode(ctx, outfmt.ModeJSON)

	cmd := newGmailBatchModifyCmd(flags)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"msg1", "msg2"})
	// Not setting --add or --remove flags

	err = cmd.Execute()
	if err == nil {
		t.Fatalf("expected error when no labels specified")
	}
	if !strings.Contains(err.Error(), "must specify --add and/or --remove") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGmailBatchDeleteCmd_TableOutput(t *testing.T) {
	origNew := newGmailService
	t.Cleanup(func() { newGmailService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/messages/batchDelete") && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.NotFound(w, r)
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

	flags := &rootFlags{Account: "a@b.com"}

	var stdoutBuf strings.Builder
	u, uiErr := ui.New(ui.Options{Stdout: &stdoutBuf, Stderr: io.Discard, Color: "never"})
	if uiErr != nil {
		t.Fatalf("ui.New: %v", uiErr)
	}
	ctx := ui.WithUI(context.Background(), u)
	// Not setting JSON mode - should output table format

	cmd := newGmailBatchDeleteCmd(flags)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"msg1", "msg2", "msg3"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if !strings.Contains(stdoutBuf.String(), "Deleted 3 messages") {
		t.Fatalf("expected 'Deleted 3 messages' in output, got: %q", stdoutBuf.String())
	}
}
