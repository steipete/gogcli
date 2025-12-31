package googleauth

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/steipete/gogcli/internal/config"
)

type AuthorizeOptions struct {
	Services     []Service
	Scopes       []string
	Manual       bool
	ForceConsent bool
	Timeout      time.Duration
}

// postSuccessDisplaySeconds is the number of seconds the success page remains
// visible before the local OAuth server shuts down.
const postSuccessDisplaySeconds = 30

// successTemplateData holds data passed to the success page template.
type successTemplateData struct {
	Email            string
	Services         []string
	CountdownSeconds int
}

var (
	readClientCredentials = config.ReadClientCredentials
	openBrowserFn         = openBrowser
	oauthEndpoint         = google.Endpoint
	randomStateFn         = randomState
)

func Authorize(ctx context.Context, opts AuthorizeOptions) (string, error) {
	if opts.Timeout <= 0 {
		opts.Timeout = 2 * time.Minute
	}
	if len(opts.Scopes) == 0 {
		return "", errors.New("missing scopes")
	}
	creds, err := readClientCredentials()
	if err != nil {
		return "", err
	}

	state, err := randomStateFn()
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	if opts.Manual {
		redirectURI := "http://localhost:1"
		cfg := oauth2.Config{
			ClientID:     creds.ClientID,
			ClientSecret: creds.ClientSecret,
			Endpoint:     oauthEndpoint,
			RedirectURL:  redirectURI,
			Scopes:       opts.Scopes,
		}
		authURL := cfg.AuthCodeURL(state, authURLParams(opts.ForceConsent)...)
		fmt.Fprintln(os.Stderr, "Visit this URL to authorize:")
		fmt.Fprintln(os.Stderr, authURL)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "After authorizing, you'll be redirected to a localhost URL that won't load.")
		fmt.Fprintln(os.Stderr, "Copy the URL from your browser's address bar and paste it here.")
		fmt.Fprintln(os.Stderr)

		fmt.Fprint(os.Stderr, "Paste redirect URL: ")
		line, readErr := bufio.NewReader(os.Stdin).ReadString('\n')
		if readErr != nil && !errors.Is(readErr, os.ErrClosed) {
			return "", readErr
		}
		line = strings.TrimSpace(line)
		code, gotState, parseErr := extractCodeAndState(line)
		if parseErr != nil {
			return "", parseErr
		}
		if gotState != "" && gotState != state {
			return "", errors.New("state mismatch")
		}
		tok, exchangeErr := cfg.Exchange(ctx, code)
		if exchangeErr != nil {
			return "", exchangeErr
		}
		if tok.RefreshToken == "" {
			return "", errors.New("no refresh token received; try again with --force-consent")
		}
		return tok.RefreshToken, nil
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer func() { _ = ln.Close() }()

	port := ln.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/oauth2/callback", port)

	cfg := oauth2.Config{
		ClientID:     creds.ClientID,
		ClientSecret: creds.ClientSecret,
		Endpoint:     oauthEndpoint,
		RedirectURL:  redirectURI,
		Scopes:       opts.Scopes,
	}

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/oauth2/callback" {
				http.NotFound(w, r)
				return
			}
			q := r.URL.Query()
			w.Header().Set("Content-Type", "text/html; charset=utf-8")

			if q.Get("error") != "" {
				select {
				case errCh <- fmt.Errorf("authorization error: %s", q.Get("error")):
				default:
				}
				w.WriteHeader(http.StatusOK)
				renderCancelledPage(w)
				return
			}
			if q.Get("state") != state {
				select {
				case errCh <- errors.New("state mismatch"):
				default:
				}
				w.WriteHeader(http.StatusBadRequest)
				renderErrorPage(w, "State mismatch - possible CSRF attack. Please try again.")
				return
			}
			code := q.Get("code")
			if code == "" {
				select {
				case errCh <- errors.New("missing code"):
				default:
				}
				w.WriteHeader(http.StatusBadRequest)
				renderErrorPage(w, "Missing authorization code. Please try again.")
				return
			}
			select {
			case codeCh <- code:
			default:
			}
			w.WriteHeader(http.StatusOK)
			renderSuccessPage(w)
		}),
	}

	go func() {
		<-ctx.Done()
		_ = srv.Close()
	}()

	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			select {
			case errCh <- err:
			default:
			}
		}
	}()

	authURL := cfg.AuthCodeURL(state, authURLParams(opts.ForceConsent)...)
	fmt.Fprintln(os.Stderr, "Opening browser for authorization…")
	fmt.Fprintln(os.Stderr, "If the browser doesn't open, visit this URL:")
	fmt.Fprintln(os.Stderr, authURL)
	_ = openBrowserFn(authURL)

	select {
	case code := <-codeCh:
		tok, exchangeErr := cfg.Exchange(ctx, code)
		if exchangeErr != nil {
			_ = srv.Close()
			return "", exchangeErr
		}
		if tok.RefreshToken == "" {
			_ = srv.Close()
			return "", errors.New("no refresh token received; try again with --force-consent")
		}
		// Keep server running so CLI waits for the user to finish auth flow (Ctrl+C ok).
		waitPostSuccess(ctx, postSuccessDisplaySeconds*time.Second)
		_ = srv.Close()
		return tok.RefreshToken, nil
	case err := <-errCh:
		_ = srv.Close()
		return "", err
	case <-ctx.Done():
		_ = srv.Close()
		return "", ctx.Err()
	}
}

func authURLParams(forceConsent bool) []oauth2.AuthCodeOption {
	opts := []oauth2.AuthCodeOption{
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("include_granted_scopes", "true"),
	}
	if forceConsent {
		opts = append(opts, oauth2.SetAuthURLParam("prompt", "consent"))
	}
	return opts
}

func randomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func extractCodeAndState(rawURL string) (code string, state string, err error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", "", err
	}
	q := parsed.Query()
	code = q.Get("code")
	if code == "" {
		return "", "", errors.New("no code found in URL")
	}
	return code, q.Get("state"), nil
}

// renderSuccessPage renders the success HTML template
func renderSuccessPage(w http.ResponseWriter) {
	tmpl, err := template.New("success").Parse(successTemplate)
	if err != nil {
		_, _ = w.Write([]byte("Success! You can close this window."))
		return
	}
	data := successTemplateData{
		CountdownSeconds: postSuccessDisplaySeconds,
	}
	_ = tmpl.Execute(w, data)
}

// renderErrorPage renders the error HTML template with the given message
func renderErrorPage(w http.ResponseWriter, errorMsg string) {
	tmpl, err := template.New("error").Parse(errorTemplate)
	if err != nil {
		_, _ = w.Write([]byte("Error: " + errorMsg))
		return
	}
	_ = tmpl.Execute(w, struct{ Error string }{Error: errorMsg})
}

// renderCancelledPage renders the cancelled HTML template
func renderCancelledPage(w http.ResponseWriter) {
	tmpl, err := template.New("cancelled").Parse(cancelledTemplate)
	if err != nil {
		_, _ = w.Write([]byte("Authorization cancelled. You can close this window."))
		return
	}
	_ = tmpl.Execute(w, nil)
}

// waitPostSuccess waits for the specified duration or until the context is
// cancelled (e.g., via Ctrl+C). This allows the success page to remain visible
// while still supporting graceful early termination.
func waitPostSuccess(ctx context.Context, d time.Duration) {
	select {
	case <-time.After(d):
	case <-ctx.Done():
	}
}
