package googleapi

import (
	"context"

	"google.golang.org/api/sheets/v4"

	"github.com/steipete/gogcli/internal/googleauth"
)

func NewSheets(ctx context.Context, email string) (*sheets.Service, error) {
	opts, err := optionsForAccount(ctx, googleauth.ServiceSheets, email)
	if err != nil {
		return nil, err
	}
	return sheets.NewService(ctx, opts...)
}
