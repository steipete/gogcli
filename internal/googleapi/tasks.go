package googleapi

import (
	"context"

	"github.com/steipete/gogcli/internal/googleauth"
	"google.golang.org/api/tasks/v1"
)

func NewTasks(ctx context.Context, email string) (*tasks.Service, error) {
	opts, err := optionsForAccount(ctx, googleauth.ServiceTasks, email)
	if err != nil {
		return nil, err
	}
	return tasks.NewService(ctx, opts...)
}
