package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/steipete/gogcli/internal/outfmt"
	"github.com/steipete/gogcli/internal/tracking"
	"github.com/steipete/gogcli/internal/ui"
)

type GmailTrackOpensCmd struct {
	TrackingID string `arg:"" optional:"" help:"Tracking ID from send command"`
	To         string `name:"to" help:"Filter by recipient email"`
	Since      string `name:"since" help:"Filter by time (e.g., '24h', '2024-01-01')"`
}

func (c *GmailTrackOpensCmd) Run(ctx context.Context, flags *RootFlags) error {
	u := ui.FromContext(ctx)

	cfg, err := tracking.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if !cfg.IsConfigured() {
		return fmt.Errorf("tracking not configured; run 'gog gmail track setup' first")
	}

	// Query by tracking ID
	if c.TrackingID != "" {
		return c.queryByTrackingID(ctx, cfg, u)
	}

	// Query via admin endpoint
	return c.queryAdmin(ctx, cfg, u, flags)
}

func (c *GmailTrackOpensCmd) queryByTrackingID(ctx context.Context, cfg *tracking.Config, u *ui.UI) error {
	reqURL := fmt.Sprintf("%s/q/%s", cfg.WorkerURL, c.TrackingID)

	resp, err := http.Get(reqURL)
	if err != nil {
		return fmt.Errorf("query tracker: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("tracker returned %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Recipient      string `json:"recipient"`
		SentAt         string `json:"sent_at"`
		TotalOpens     int    `json:"total_opens"`
		HumanOpens     int    `json:"human_opens"`
		FirstHumanOpen *struct {
			At       string `json:"at"`
			Location *struct {
				City    string `json:"city"`
				Region  string `json:"region"`
				Country string `json:"country"`
			} `json:"location"`
		} `json:"first_human_open"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	u.Out().Printf("Recipient: %s", result.Recipient)
	u.Out().Printf("Sent: %s", result.SentAt)
	u.Out().Printf("Opens: %d total, %d human", result.TotalOpens, result.HumanOpens)

	if result.FirstHumanOpen != nil {
		loc := "unknown location"
		if result.FirstHumanOpen.Location != nil && result.FirstHumanOpen.Location.City != "" {
			loc = fmt.Sprintf("%s, %s", result.FirstHumanOpen.Location.City, result.FirstHumanOpen.Location.Region)
		}
		u.Out().Printf("First opened: %s · %s", result.FirstHumanOpen.At, loc)
	}

	return nil
}

func (c *GmailTrackOpensCmd) queryAdmin(ctx context.Context, cfg *tracking.Config, u *ui.UI, flags *RootFlags) error {
	reqURL, _ := url.Parse(cfg.WorkerURL + "/opens")
	q := reqURL.Query()
	if c.To != "" {
		q.Set("recipient", c.To)
	}
	if c.Since != "" {
		q.Set("since", c.Since)
	}
	reqURL.RawQuery = q.Encode()

	req, _ := http.NewRequestWithContext(ctx, "GET", reqURL.String(), nil)
	req.Header.Set("Authorization", "Bearer "+cfg.AdminKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("query tracker: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return fmt.Errorf("unauthorized: admin key may be incorrect")
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("tracker returned %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Opens []struct {
			Recipient   string `json:"recipient"`
			SubjectHash string `json:"subject_hash"`
			SentAt      string `json:"sent_at"`
			OpenedAt    string `json:"opened_at"`
			IsBot       bool   `json:"is_bot"`
			Location    *struct {
				City    string `json:"city"`
				Region  string `json:"region"`
				Country string `json:"country"`
			} `json:"location"`
		} `json:"opens"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, result)
	}

	if len(result.Opens) == 0 {
		u.Out().Printf("No opens found")
		return nil
	}

	for _, o := range result.Opens {
		loc := ""
		if o.Location != nil && o.Location.City != "" {
			loc = fmt.Sprintf(" · %s, %s", o.Location.City, o.Location.Region)
		}
		botMark := ""
		if o.IsBot {
			botMark = " (bot)"
		}
		u.Out().Printf("%s  %s  %s%s%s", o.Recipient, o.SentAt[:10], o.OpenedAt, loc, botMark)
	}

	return nil
}
