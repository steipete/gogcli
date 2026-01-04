package cmd

import (
	"context"
	"fmt"

	"github.com/steipete/gogcli/internal/tracking"
	"github.com/steipete/gogcli/internal/ui"
)

type GmailTrackStatusCmd struct{}

func (c *GmailTrackStatusCmd) Run(ctx context.Context, flags *RootFlags) error {
	u := ui.FromContext(ctx)

	cfg, err := tracking.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if !cfg.IsConfigured() {
		u.Out().Printf("Tracking: not configured")
		u.Out().Printf("")
		u.Out().Printf("Run 'gog gmail track setup' to enable email tracking.")
		return nil
	}

	u.Out().Printf("Tracking: enabled")
	u.Out().Printf("Tracker URL: %s", cfg.WorkerURL)

	return nil
}
