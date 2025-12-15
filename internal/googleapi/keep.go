package googleapi

import (
	"context"

	"github.com/steipete/gogcli/internal/googleauth"
	"google.golang.org/api/keep/v1"
)

func NewKeep(ctx context.Context, email string) (*keep.Service, error) {
	opts, err := optionsForAccount(ctx, googleauth.ServiceKeep, email)
	if err != nil {
		return nil, err
	}
	return keep.NewService(ctx, opts...)
}
