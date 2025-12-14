package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/steipete/gogcli/internal/outfmt"
	"github.com/steipete/gogcli/internal/ui"
	"google.golang.org/api/gmail/v1"
)

func newGmailForwardingCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "forwarding",
		Short: "Manage email forwarding addresses",
		Long: `Manage email forwarding addresses.

Forwarding addresses must be verified before they can be used. Creating a forwarding address
sends a verification email to the target address that must be confirmed.`,
	}

	cmd.AddCommand(newGmailForwardingListCmd(flags))
	cmd.AddCommand(newGmailForwardingGetCmd(flags))
	cmd.AddCommand(newGmailForwardingCreateCmd(flags))
	cmd.AddCommand(newGmailForwardingDeleteCmd(flags))
	return cmd
}

func newGmailForwardingListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all forwarding addresses",
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

			resp, err := svc.Users.Settings.ForwardingAddresses.List("me").Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"forwardingAddresses": resp.ForwardingAddresses})
			}

			if len(resp.ForwardingAddresses) == 0 {
				u.Err().Println("No forwarding addresses")
				return nil
			}

			tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "EMAIL\tSTATUS")
			for _, f := range resp.ForwardingAddresses {
				fmt.Fprintf(tw, "%s\t%s\n",
					f.ForwardingEmail,
					f.VerificationStatus)
			}
			_ = tw.Flush()
			return nil
		},
	}
}

func newGmailForwardingGetCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get <forwardingEmail>",
		Short: "Get a specific forwarding address",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}

			svc, err := newGmailService(cmd.Context(), account)
			if err != nil {
				return err
			}

			forwardingEmail := args[0]
			address, err := svc.Users.Settings.ForwardingAddresses.Get("me", forwardingEmail).Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"forwardingAddress": address})
			}

			u.Out().Printf("forwarding_email\t%s", address.ForwardingEmail)
			u.Out().Printf("verification_status\t%s", address.VerificationStatus)
			return nil
		},
	}
}

func newGmailForwardingCreateCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "create <forwardingEmail>",
		Short: "Create/add a forwarding address",
		Long: `Create/add a forwarding address.

This sends a verification email to the target address. The forwarding address
cannot be used until the recipient clicks the verification link in the email.

The verification status will be "pending" until confirmed, then "accepted".`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}

			svc, err := newGmailService(cmd.Context(), account)
			if err != nil {
				return err
			}

			forwardingEmail := args[0]
			address := &gmail.ForwardingAddress{
				ForwardingEmail: forwardingEmail,
			}

			created, err := svc.Users.Settings.ForwardingAddresses.Create("me", address).Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"forwardingAddress": created})
			}

			u.Out().Println("Forwarding address created successfully")
			u.Out().Printf("forwarding_email\t%s", created.ForwardingEmail)
			u.Out().Printf("verification_status\t%s", created.VerificationStatus)
			u.Out().Println("\nA verification email has been sent to the forwarding address.")
			u.Out().Println("The address cannot be used until the recipient confirms the verification link.")
			return nil
		},
	}
}

func newGmailForwardingDeleteCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <forwardingEmail>",
		Short: "Delete a forwarding address",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}

			svc, err := newGmailService(cmd.Context(), account)
			if err != nil {
				return err
			}

			forwardingEmail := args[0]
			err = svc.Users.Settings.ForwardingAddresses.Delete("me", forwardingEmail).Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{
					"success":        true,
					"forwardingEmail": forwardingEmail,
				})
			}

			u.Out().Printf("Forwarding address %s deleted successfully", forwardingEmail)
			return nil
		},
	}
}
