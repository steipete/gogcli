package tracking

import (
	"strings"
	"testing"
)

func TestGeneratePixelURL(t *testing.T) {
	key, _ := GenerateKey()
	cfg := &Config{
		Enabled:     true,
		WorkerURL:   "https://test.workers.dev",
		TrackingKey: key,
	}

	pixelURL, blob, err := GeneratePixelURL(cfg, "test@example.com", "Hello World")
	if err != nil {
		t.Fatalf("GeneratePixelURL failed: %v", err)
	}

	if hasPrefix := strings.HasPrefix(pixelURL, "https://test.workers.dev/p/"); !hasPrefix {
		t.Errorf("Unexpected URL prefix: %s", pixelURL)
	}

	if hasSuffix := strings.HasSuffix(pixelURL, ".gif"); !hasSuffix {
		t.Errorf("URL should end with .gif: %s", pixelURL)
	}

	if isEmpty := blob == ""; isEmpty {
		t.Error("Blob should not be empty")
	}
}

func TestGeneratePixelURLNotConfigured(t *testing.T) {
	cfg := &Config{Enabled: false}

	_, _, err := GeneratePixelURL(cfg, "test@example.com", "Hello")
	if err == nil {
		t.Error("Expected error for unconfigured tracking")
	}
}

func TestGeneratePixelHTML(t *testing.T) {
	html := GeneratePixelHTML("https://test.workers.dev/p/abc123.gif")

	if hasSrc := strings.Contains(html, `src="https://test.workers.dev/p/abc123.gif"`); !hasSrc {
		t.Errorf("HTML missing src: %s", html)
	}

	if hasWidth := strings.Contains(html, `width="1"`); !hasWidth {
		t.Errorf("HTML missing width: %s", html)
	}

	if hasStyle := strings.Contains(html, `style="display:none`); !hasStyle {
		t.Errorf("HTML missing display:none: %s", html)
	}
}

func TestHashSubjectConsistent(t *testing.T) {
	h1 := hashSubject("Hello World")
	if h2 := hashSubject("Hello World"); h1 != h2 {
		t.Error("Same subject should produce same hash")
	}

	if h3 := hashSubject("Different Subject"); h1 == h3 {
		t.Error("Different subjects should produce different hashes")
	}

	if hashLen := len(h1); hashLen != 6 {
		t.Errorf("Hash should be 6 chars, got %d", hashLen)
	}
}
