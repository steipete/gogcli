package cmd

import (
	"context"
	"fmt"

	"github.com/steipete/gogcli/internal/ui"
)

type GmailTrackSetupCmd struct {
	Domain string `name:"domain" help:"Custom tracking domain (optional)"`
}

func (c *GmailTrackSetupCmd) Run(ctx context.Context, flags *RootFlags) error {
	u := ui.FromContext(ctx)

	u.Out().Printf("Email Tracking Setup")
	u.Out().Printf("====================")
	u.Out().Printf("")
	u.Out().Printf("This feature requires wrangler CLI and a Cloudflare account.")
	u.Out().Printf("")
	u.Out().Printf("Setup steps:")
	u.Out().Printf("1. Install wrangler: npm install -g wrangler")
	u.Out().Printf("2. Login to Cloudflare: wrangler login")
	u.Out().Printf("3. Deploy the worker from internal/tracking/worker/")
	u.Out().Printf("")
	u.Out().Printf("Full automated setup coming soon.")

	return fmt.Errorf("automated setup not yet implemented")
}
