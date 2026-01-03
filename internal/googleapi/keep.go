package googleapi

import (
	"context"
	"fmt"
	"os"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/keep/v1"
	"google.golang.org/api/option"

	"github.com/steipete/gogcli/internal/googleauth"
)

func NewKeep(ctx context.Context, email string) (*keep.Service, error) {
	if opts, err := optionsForAccount(ctx, googleauth.ServiceKeep, email); err != nil {
		return nil, fmt.Errorf("keep options: %w", err)
	} else if svc, err := keep.NewService(ctx, opts...); err != nil {
		return nil, fmt.Errorf("create keep service: %w", err)
	} else {
		return svc, nil
	}
}

func NewKeepWithServiceAccount(ctx context.Context, serviceAccountPath, impersonateEmail string) (*keep.Service, error) {
	data, err := os.ReadFile(serviceAccountPath)
	if err != nil {
		return nil, fmt.Errorf("read service account file: %w", err)
	}

	config, err := google.JWTConfigFromJSON(data, "https://www.googleapis.com/auth/keep")
	if err != nil {
		return nil, fmt.Errorf("parse service account: %w", err)
	}

	config.Subject = impersonateEmail

	svc, err := keep.NewService(ctx, option.WithTokenSource(config.TokenSource(ctx)))
	if err != nil {
		return nil, fmt.Errorf("create keep service: %w", err)
	}

	return svc, nil
}
