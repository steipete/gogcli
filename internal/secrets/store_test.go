package secrets

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/99designs/keyring"
)

func TestTokenKey(t *testing.T) {
	if got := tokenKey("a@b.com"); got != "token:a@b.com" {
		t.Fatalf("unexpected: %q", got)
	}
}

func TestParseTokenKey(t *testing.T) {
	email, ok := ParseTokenKey("token:a@b.com")
	if !ok {
		t.Fatalf("expected ok")
	}

	if email != "a@b.com" {
		t.Fatalf("unexpected: %q", email)
	}

	if _, ok := ParseTokenKey("nope"); ok {
		t.Fatalf("expected not ok")
	}
}

func TestKeyringStore_TokenRoundtrip(t *testing.T) {
	s := &KeyringStore{ring: keyring.NewArrayKeyring(nil)}

	createdAt := time.Date(2025, 12, 12, 0, 0, 0, 0, time.UTC)
	if err := s.SetToken("A@B.COM", Token{
		Email:        "A@B.COM",
		Services:     []string{"gmail"},
		Scopes:       []string{"scope1"},
		CreatedAt:    createdAt,
		RefreshToken: "rt",
	}); err != nil {
		t.Fatalf("SetToken: %v", err)
	}

	got, err := s.GetToken("a@b.com")
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}

	if got.Email != "a@b.com" {
		t.Fatalf("email: %q", got.Email)
	}

	if got.RefreshToken != "rt" {
		t.Fatalf("refresh token mismatch")
	}

	if got.CreatedAt.IsZero() {
		t.Fatalf("expected createdAt")
	}

	list, err := s.ListTokens()
	if err != nil {
		t.Fatalf("ListTokens: %v", err)
	}

	if len(list) != 1 || list[0].Email != "a@b.com" {
		t.Fatalf("unexpected list: %#v", list)
	}

	if err := s.DeleteToken("a@b.com"); err != nil {
		t.Fatalf("DeleteToken: %v", err)
	}

	if _, err := s.GetToken("a@b.com"); err == nil {
		t.Fatalf("expected error after delete")
	}
}

func TestKeyringStore_DefaultAccount_Roundtrip(t *testing.T) {
	s := &KeyringStore{ring: keyring.NewArrayKeyring(nil)}

	// Missing default should be empty without error.
	if got, err := s.GetDefaultAccount(); err != nil || got != "" {
		t.Fatalf("unexpected default: %q err=%v", got, err)
	}

	if err := s.SetDefaultAccount("A@B.COM"); err != nil {
		t.Fatalf("SetDefaultAccount: %v", err)
	}

	got, err := s.GetDefaultAccount()
	if err != nil {
		t.Fatalf("GetDefaultAccount: %v", err)
	}

	if got != "a@b.com" {
		t.Fatalf("unexpected default: %q", got)
	}
}

var (
	errTestKeychainLocked = errors.New("store token: User Interaction is not allowed. (-25308)")
	errTestOther          = errors.New("store token: some other error")
)

func TestWrapKeychainError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantWrapped bool
		wantContain string
	}{
		{
			name:        "nil error",
			err:         nil,
			wantWrapped: false,
		},
		{
			name:        "keychain locked error",
			err:         errTestKeychainLocked,
			wantWrapped: true,
			wantContain: "security unlock-keychain",
		},
		{
			name:        "other error",
			err:         errTestOther,
			wantWrapped: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := wrapKeychainError(tt.err)

			if tt.err == nil {
				if wrapped != nil {
					t.Error("expected nil error to remain nil")
				}

				return
			}

			if tt.wantWrapped {
				if errors.Is(wrapped, tt.err) && wrapped.Error() == tt.err.Error() {
					t.Error("expected error to be wrapped with additional context")
				}

				if tt.wantContain != "" && !strings.Contains(wrapped.Error(), tt.wantContain) {
					t.Errorf("wrapped error %q should contain %q", wrapped.Error(), tt.wantContain)
				}
			} else if !errors.Is(wrapped, tt.err) {
				t.Error("expected error to remain unchanged")
			}
		})
	}
}
