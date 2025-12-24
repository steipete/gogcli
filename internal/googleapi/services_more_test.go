package googleapi

import (
	"context"
	"errors"
	"testing"

	"github.com/99designs/keyring"
	"github.com/steipete/gogcli/internal/config"
	"github.com/steipete/gogcli/internal/secrets"
)

func TestNewServices_HappyPath(t *testing.T) {
	origRead := readClientCredentials
	origOpen := openSecretsStore
	t.Cleanup(func() {
		readClientCredentials = origRead
		openSecretsStore = origOpen
	})

	readClientCredentials = func() (config.ClientCredentials, error) {
		return config.ClientCredentials{ClientID: "id", ClientSecret: "secret"}, nil
	}
	openSecretsStore = func() (secrets.Store, error) {
		return &stubStore{tok: secrets.Token{Email: "a@b.com", RefreshToken: "rt"}}, nil
	}

	ctx := context.Background()
	if svc, err := NewGmail(ctx, "a@b.com"); err != nil || svc == nil {
		t.Fatalf("NewGmail: %v", err)
	}
	if svc, err := NewDrive(ctx, "a@b.com"); err != nil || svc == nil {
		t.Fatalf("NewDrive: %v", err)
	}
	if svc, err := NewCalendar(ctx, "a@b.com"); err != nil || svc == nil {
		t.Fatalf("NewCalendar: %v", err)
	}
	if svc, err := NewPeopleContacts(ctx, "a@b.com"); err != nil || svc == nil {
		t.Fatalf("NewPeopleContacts: %v", err)
	}
	if svc, err := NewPeopleOtherContacts(ctx, "a@b.com"); err != nil || svc == nil {
		t.Fatalf("NewPeopleOtherContacts: %v", err)
	}
	if svc, err := NewPeopleDirectory(ctx, "a@b.com"); err != nil || svc == nil {
		t.Fatalf("NewPeopleDirectory: %v", err)
	}
	if svc, err := NewTasks(ctx, "a@b.com"); err != nil || svc == nil {
		t.Fatalf("NewTasks: %v", err)
	}
}

func TestNewServices_AuthRequired(t *testing.T) {
	origRead := readClientCredentials
	origOpen := openSecretsStore
	t.Cleanup(func() {
		readClientCredentials = origRead
		openSecretsStore = origOpen
	})

	readClientCredentials = func() (config.ClientCredentials, error) {
		return config.ClientCredentials{ClientID: "id", ClientSecret: "secret"}, nil
	}
	openSecretsStore = func() (secrets.Store, error) {
		return &stubStore{err: keyring.ErrKeyNotFound}, nil
	}

	_, err := NewGmail(context.Background(), "a@b.com")
	if err == nil {
		t.Fatalf("expected error")
	}
	var are *AuthRequiredError
	if !errors.As(err, &are) {
		t.Fatalf("expected AuthRequiredError, got: %T %v", err, err)
	}
}
