package cmd

import (
	"errors"
	"os"

	"github.com/spf13/cobra"
	"github.com/steipete/gogcli/internal/outfmt"
	"github.com/steipete/gogcli/internal/ui"
	"google.golang.org/api/gmail/v1"
)

func newGmailAutoForwardCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "autoforward",
		Short: "Manage auto-forwarding settings",
		Long: `Manage auto-forwarding settings.

The email address must first be verified via 'gmail forwarding create' before it can be used
for auto-forwarding.`,
	}

	cmd.AddCommand(newGmailAutoForwardGetCmd(flags))
	cmd.AddCommand(newGmailAutoForwardUpdateCmd(flags))
	return cmd
}

func newGmailAutoForwardGetCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "Get current auto-forwarding settings",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}

			svc, err := newGmailService(cmd.Context(), account)
			if err != nil {
				return err
			}

			autoForward, err := svc.Users.Settings.GetAutoForwarding("me").Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"autoForwarding": autoForward})
			}

			u.Out().Printf("enabled\t%t", autoForward.Enabled)
			if autoForward.EmailAddress != "" {
				u.Out().Printf("email_address\t%s", autoForward.EmailAddress)
			}
			if autoForward.Disposition != "" {
				u.Out().Printf("disposition\t%s", autoForward.Disposition)
			}
			return nil
		},
	}
}

func newGmailAutoForwardUpdateCmd(flags *rootFlags) *cobra.Command {
	var enable bool
	var disable bool
	var email string
	var disposition string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update auto-forwarding settings",
		Long: `Update auto-forwarding settings.

The email address must first be verified via 'gmail forwarding create' before it can be used.

Valid disposition values:
  - leaveInInbox: Leave forwarded messages in inbox
  - archive: Archive forwarded messages
  - trash: Move forwarded messages to trash
  - markRead: Mark forwarded messages as read`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}

			if enable && disable {
				return errors.New("cannot specify both --enable and --disable")
			}

			svc, err := newGmailService(cmd.Context(), account)
			if err != nil {
				return err
			}

			// Get current settings first
			current, err := svc.Users.Settings.GetAutoForwarding("me").Do()
			if err != nil {
				return err
			}

			// Build update request, preserving existing values if not specified
			autoForward := &gmail.AutoForwarding{
				Enabled:      current.Enabled,
				EmailAddress: current.EmailAddress,
				Disposition:  current.Disposition,
			}

			// Apply flags
			if enable {
				autoForward.Enabled = true
			}
			if disable {
				autoForward.Enabled = false
			}
			if cmd.Flags().Changed("email") {
				autoForward.EmailAddress = email
			}
			if cmd.Flags().Changed("disposition") {
				// Validate disposition value
				validDispositions := map[string]bool{
					"leaveInInbox": true,
					"archive":      true,
					"trash":        true,
					"markRead":     true,
				}
				if !validDispositions[disposition] {
					return errors.New("invalid disposition value; must be one of: leaveInInbox, archive, trash, markRead")
				}
				autoForward.Disposition = disposition
			}

			updated, err := svc.Users.Settings.UpdateAutoForwarding("me", autoForward).Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"autoForwarding": updated})
			}

			u.Out().Println("Auto-forwarding settings updated successfully")
			u.Out().Printf("enabled\t%t", updated.Enabled)
			if updated.EmailAddress != "" {
				u.Out().Printf("email_address\t%s", updated.EmailAddress)
			}
			if updated.Disposition != "" {
				u.Out().Printf("disposition\t%s", updated.Disposition)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&enable, "enable", false, "Enable auto-forwarding")
	cmd.Flags().BoolVar(&disable, "disable", false, "Disable auto-forwarding")
	cmd.Flags().StringVar(&email, "email", "", "Email address to forward to (must be verified first)")
	cmd.Flags().StringVar(&disposition, "disposition", "", "What to do with forwarded messages: leaveInInbox, archive, trash, markRead")
	return cmd
}
