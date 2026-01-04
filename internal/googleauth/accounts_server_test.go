package googleauth

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"html/template"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/steipete/gogcli/internal/config"
	"github.com/steipete/gogcli/internal/secrets"
)

var errBoom = errors.New("boom")

type fakeStore struct {
	tokens       []secrets.Token
	defaultEmail string

	setTokenEmail    string
	setTokenValue    secrets.Token
	setTokenErr      error
	setDefaultCalled string
	setDefaultErr    error
	deleteCalled     string
	deleteErr        error
	listErr          error
}

func (s *fakeStore) Keys() ([]string, error) { return nil, nil }
func (s *fakeStore) SetToken(email string, tok secrets.Token) error {
	s.setTokenEmail = email
	s.setTokenValue = tok

	if s.setTokenErr != nil {
		return s.setTokenErr
	}

	return nil
}
func (s *fakeStore) GetToken(string) (secrets.Token, error) { return secrets.Token{}, nil }
func (s *fakeStore) DeleteToken(email string) error {
	s.deleteCalled = email
	if s.deleteErr != nil {
		return s.deleteErr
	}

	return nil
}

func (s *fakeStore) ListTokens() ([]secrets.Token, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}

	return append([]secrets.Token(nil), s.tokens...), nil
}
func (s *fakeStore) GetDefaultAccount() (string, error) { return s.defaultEmail, nil }
func (s *fakeStore) SetDefaultAccount(email string) error {
	s.setDefaultCalled = email
	s.defaultEmail = email

	if s.setDefaultErr != nil {
		return s.setDefaultErr
	}

	return nil
}

func TestManageServer_HandleAccountsPage(t *testing.T) {
	ms := &ManageServer{
		csrfToken: "csrf",
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ms.handleAccountsPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}

	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("content-type: %q", ct)
	}

	if body := rr.Body.String(); strings.TrimSpace(body) == "" {
		tmpl, err := template.New("accounts").Parse(accountsTemplate)
		if err != nil {
			t.Fatalf("expected body, parse err=%v", err)
		}
		var buf bytes.Buffer
		execErr := tmpl.Execute(&buf, struct{ CSRFToken string }{CSRFToken: "csrf"})

		t.Fatalf("expected body; handler wrote 0 bytes; direct execute bytes=%d err=%v", buf.Len(), execErr)
	} else {
		if !strings.Contains(body, "csrfToken") || !strings.Contains(body, "const csrfToken") {
			t.Fatalf("expected csrf js in body")
		}

		if !strings.Contains(body, "'csrf'") && !strings.Contains(body, "\"csrf\"") {
			excerpt := body
			if len(excerpt) > 200 {
				excerpt = excerpt[:200]
			}

			t.Fatalf("expected rendered token, body excerpt=%q", excerpt)
		}
	}
}

func TestManageServer_HandleListAccounts_DefaultFirst(t *testing.T) {
	store := &fakeStore{
		tokens: []secrets.Token{
			{Email: "a@b.com", Services: []string{"gmail"}},
			{Email: "c@d.com", Services: []string{"drive"}},
		},
	}
	ms := &ManageServer{
		csrfToken: "csrf",
		store:     store,
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/accounts", nil)
	ms.handleListAccounts(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	var parsed struct {
		Accounts []AccountInfo `json:"accounts"`
	}

	if err := json.Unmarshal(rr.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("json parse: %v", err)
	}

	if len(parsed.Accounts) != 2 || !parsed.Accounts[0].IsDefault || parsed.Accounts[1].IsDefault {
		t.Fatalf("unexpected defaults: %#v", parsed.Accounts)
	}
}

func TestManageServer_HandleListAccounts_DefaultExplicit(t *testing.T) {
	store := &fakeStore{
		tokens: []secrets.Token{
			{Email: "a@b.com", Services: []string{"gmail"}},
			{Email: "c@d.com", Services: []string{"drive"}},
		},
		defaultEmail: "c@d.com",
	}
	ms := &ManageServer{
		csrfToken: "csrf",
		store:     store,
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/accounts", nil)
	ms.handleListAccounts(rr, req)

	var parsed struct {
		Accounts []AccountInfo `json:"accounts"`
	}

	if err := json.Unmarshal(rr.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("json parse: %v", err)
	}

	if len(parsed.Accounts) != 2 || parsed.Accounts[0].IsDefault || !parsed.Accounts[1].IsDefault {
		t.Fatalf("unexpected defaults: %#v", parsed.Accounts)
	}
}

func TestManageServer_HandleOAuthCallback_ErrorAndValidation(t *testing.T) {
	ms := &ManageServer{
		csrfToken:  "csrf",
		oauthState: "state1",
	}
	// Need a listener for redirectURI generation even though we don't reach exchange.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	t.Cleanup(func() { _ = ln.Close() })
	ms.listener = ln

	t.Run("cancelled", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/oauth2/callback?error=access_denied", nil)
		ms.handleOAuthCallback(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status: %d", rr.Code)
		}
	})

	t.Run("state mismatch", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/oauth2/callback?state=nope&code=abc", nil)
		ms.handleOAuthCallback(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status: %d", rr.Code)
		}
	})

	t.Run("missing code", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/oauth2/callback?state=state1", nil)
		ms.handleOAuthCallback(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status: %d", rr.Code)
		}
	})
}

func TestManageServer_HandleSetDefault_AndRemove(t *testing.T) {
	store := &fakeStore{
		tokens: []secrets.Token{{Email: "a@b.com"}},
	}
	ms := &ManageServer{
		csrfToken: "csrf",
		store:     store,
	}

	t.Run("set-default csrf", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/set-default", bytes.NewReader([]byte(`{"email":"a@b.com"}`)))
		req.Header.Set("X-CSRF-Token", "nope")
		ms.handleSetDefault(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Fatalf("status: %d", rr.Code)
		}
	})

	t.Run("set-default ok", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/set-default", bytes.NewReader([]byte(`{"email":"a@b.com"}`)))
		req.Header.Set("X-CSRF-Token", "csrf")
		ms.handleSetDefault(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
		}

		if store.setDefaultCalled != "a@b.com" {
			t.Fatalf("expected setDefaultCalled")
		}
	})

	t.Run("set-default bad method", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/set-default", nil)
		ms.handleSetDefault(rr, req)

		if rr.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status: %d", rr.Code)
		}
	})

	t.Run("set-default bad json", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/set-default", bytes.NewReader([]byte(`{`)))
		req.Header.Set("X-CSRF-Token", "csrf")
		ms.handleSetDefault(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status: %d", rr.Code)
		}
	})

	t.Run("set-default store error", func(t *testing.T) {
		store.setDefaultErr = errBoom

		t.Cleanup(func() { store.setDefaultErr = nil })
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/set-default", bytes.NewReader([]byte(`{"email":"a@b.com"}`)))
		req.Header.Set("X-CSRF-Token", "csrf")
		ms.handleSetDefault(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status: %d", rr.Code)
		}
	})

	t.Run("remove ok", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/remove-account", bytes.NewReader([]byte(`{"email":"a@b.com"}`)))
		req.Header.Set("X-CSRF-Token", "csrf")
		ms.handleRemoveAccount(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status: %d", rr.Code)
		}

		if store.deleteCalled != "a@b.com" {
			t.Fatalf("expected deleteCalled")
		}
	})

	t.Run("remove bad method", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/remove-account", nil)
		ms.handleRemoveAccount(rr, req)

		if rr.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status: %d", rr.Code)
		}
	})

	t.Run("remove bad json", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/remove-account", bytes.NewReader([]byte(`{`)))
		req.Header.Set("X-CSRF-Token", "csrf")
		ms.handleRemoveAccount(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status: %d", rr.Code)
		}
	})

	t.Run("remove store error", func(t *testing.T) {
		store.deleteErr = errBoom

		t.Cleanup(func() { store.deleteErr = nil })
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/remove-account", bytes.NewReader([]byte(`{"email":"a@b.com"}`)))
		req.Header.Set("X-CSRF-Token", "csrf")
		ms.handleRemoveAccount(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status: %d", rr.Code)
		}
	})
}

func TestManageServer_HandleListAccounts_Error(t *testing.T) {
	store := &fakeStore{listErr: errBoom}
	ms := &ManageServer{csrfToken: "csrf", store: store}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/accounts", nil)
	ms.handleListAccounts(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", rr.Code)
	}
}

func TestGenerateCSRFToken(t *testing.T) {
	token, err := generateCSRFToken()
	if err != nil {
		t.Fatalf("generateCSRFToken: %v", err)
	}

	if len(token) != 64 {
		t.Fatalf("unexpected token length: %d", len(token))
	}

	if _, err := hex.DecodeString(token); err != nil {
		t.Fatalf("token not hex: %v", err)
	}
}

func TestRenderSuccessPageWithDetails(t *testing.T) {
	rr := httptest.NewRecorder()
	renderSuccessPageWithDetails(rr, "me@example.com", []string{"gmail", "drive"})

	if body := rr.Body.String(); !strings.Contains(body, "me@example.com") {
		t.Fatalf("expected email in body")
	} else {
		if !strings.Contains(body, "gmail") || !strings.Contains(body, "drive") {
			t.Fatalf("expected services in body")
		}

		if !strings.Contains(body, strconv.Itoa(postSuccessDisplaySeconds)) {
			t.Fatalf("expected countdown in body")
		}
	}
}

func TestManageServer_HandleAuthStart(t *testing.T) {
	origRead := readClientCredentials
	origState := randomStateFn
	origEndpoint := oauthEndpoint

	t.Cleanup(func() {
		readClientCredentials = origRead
		randomStateFn = origState
		oauthEndpoint = origEndpoint
	})

	readClientCredentials = func() (config.ClientCredentials, error) {
		return config.ClientCredentials{ClientID: "id", ClientSecret: "secret"}, nil
	}
	randomStateFn = func() (string, error) { return "state123", nil }
	oauthEndpoint = oauth2.Endpoint{AuthURL: "http://example.com/auth", TokenURL: "http://example.com/token"}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	t.Cleanup(func() { _ = ln.Close() })

	ms := &ManageServer{listener: ln}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/start", nil)
	ms.handleAuthStart(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("status: %d", rr.Code)
	}
	loc := rr.Header().Get("Location")
	var parsed *url.URL

	if p, err := url.Parse(loc); err != nil {
		t.Fatalf("parse location: %v", err)
	} else {
		parsed = p
	}

	if parsed.Host != "example.com" {
		t.Fatalf("unexpected host: %q", parsed.Host)
	}

	if state := parsed.Query().Get("state"); state != "state123" {
		t.Fatalf("unexpected state: %q", state)
	}

	if ms.oauthState != "state123" {
		t.Fatalf("expected oauthState set")
	}

	if redirectURI := parsed.Query().Get("redirect_uri"); !strings.Contains(redirectURI, "127.0.0.1:") {
		t.Fatalf("expected redirect uri, got %q", redirectURI)
	}
}

func TestManageServer_HandleAuthStart_CredentialsError(t *testing.T) {
	origRead := readClientCredentials

	t.Cleanup(func() { readClientCredentials = origRead })

	readClientCredentials = func() (config.ClientCredentials, error) {
		return config.ClientCredentials{}, errBoom
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/start", nil)
	ms := &ManageServer{}
	ms.handleAuthStart(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", rr.Code)
	}
}

func TestManageServer_HandleOAuthCallback_Success(t *testing.T) {
	origRead := readClientCredentials
	origEndpoint := oauthEndpoint
	origFetchUserEmail := fetchUserEmailFn

	t.Cleanup(func() {
		readClientCredentials = origRead
		oauthEndpoint = origEndpoint
		fetchUserEmailFn = origFetchUserEmail
	})

	readClientCredentials = func() (config.ClientCredentials, error) {
		return config.ClientCredentials{ClientID: "id", ClientSecret: "secret"}, nil
	}

	// Mock userinfo endpoint
	userinfoSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth2/v2/userinfo" {
			t.Fatalf("unexpected path: %s", r.URL.Path)

			return
		}

		auth := r.Header.Get("Authorization")
		if auth != "Bearer token" {
			t.Fatalf("expected Bearer token, got %q", auth)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"email": "me@example.com"})
	}))
	defer userinfoSrv.Close()

	// Override fetchUserEmail to use our mock server
	fetchUserEmailFn = func(ctx context.Context, accessToken string) (string, error) {
		return fetchUserEmailWithURL(ctx, accessToken, userinfoSrv.URL+"/oauth2/v2/userinfo")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}

		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}

		if r.Form.Get("code") != "abc" {
			t.Fatalf("expected code=abc, got %q", r.Form.Get("code"))
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "token",
			"refresh_token": "refresh",
			"token_type":    "Bearer",
			"expires_in":    3600,
		})
	}))
	defer srv.Close()

	oauthEndpoint = oauth2.Endpoint{AuthURL: "http://example.com/auth", TokenURL: srv.URL}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	t.Cleanup(func() { _ = ln.Close() })

	store := &fakeStore{}
	ms := &ManageServer{
		oauthState: "state1",
		listener:   ln,
		store:      store,
		opts:       ManageServerOptions{Services: []Service{ServiceGmail}},
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/oauth2/callback?state=state1&code=abc", nil)
	ms.handleOAuthCallback(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body: %s", rr.Code, rr.Body.String())
	}

	if store.setTokenEmail != "me@example.com" {
		t.Fatalf("expected token stored for me@example.com, got %q", store.setTokenEmail)
	}

	if store.setTokenValue.RefreshToken != "refresh" {
		t.Fatalf("expected refresh token stored")
	}

	if !strings.Contains(rr.Body.String(), "me@example.com") {
		t.Fatalf("expected body to include email")
	}
}

func TestFetchUserEmail(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if auth := r.Header.Get("Authorization"); auth != "Bearer test-token" {
				t.Fatalf("expected Bearer test-token, got %q", auth)
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"email": "user@test.com"})
		}))
		defer srv.Close()

		email, err := fetchUserEmailWithURL(context.Background(), "test-token", srv.URL)
		if err != nil {
			t.Fatalf("fetchUserEmail: %v", err)
		}

		if email != "user@test.com" {
			t.Fatalf("expected user@test.com, got %q", email)
		}
	})

	t.Run("empty email", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"email": ""})
		}))
		defer srv.Close()

		_, err := fetchUserEmailWithURL(context.Background(), "test-token", srv.URL)
		if err == nil {
			t.Fatal("expected error for empty email")
		}
	})

	t.Run("http error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer srv.Close()

		_, err := fetchUserEmailWithURL(context.Background(), "test-token", srv.URL)
		if err == nil {
			t.Fatal("expected error for 401")
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("{invalid"))
		}))
		defer srv.Close()

		_, err := fetchUserEmailWithURL(context.Background(), "test-token", srv.URL)
		if err == nil {
			t.Fatal("expected error for invalid json")
		}
	})
}

func TestStartManageServer_Timeout(t *testing.T) {
	origStore := openDefaultStore
	origOpen := openBrowserFn

	t.Cleanup(func() {
		openDefaultStore = origStore
		openBrowserFn = origOpen
	})

	openDefaultStore = func() (secrets.Store, error) { return &fakeStore{}, nil }
	var opened string
	openBrowserFn = func(url string) error {
		opened = url
		return nil
	}

	ctx := context.Background()
	if err := StartManageServer(ctx, ManageServerOptions{Timeout: 50 * time.Millisecond}); err != nil {
		t.Fatalf("StartManageServer: %v", err)
	}

	if !strings.Contains(opened, "http://127.0.0.1:") {
		t.Fatalf("expected browser URL, got %q", opened)
	}
}
