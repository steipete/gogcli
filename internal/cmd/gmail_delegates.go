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

func newGmailDelegatesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delegates",
		Short: "Manage email delegation (G Suite/Workspace feature)",
		Long: `Manage email delegation settings.

Delegation allows someone else to read, send, and delete messages on your behalf.
This is a G Suite/Workspace feature and may not be available for personal Gmail accounts.`,
	}

	cmd.AddCommand(newGmailDelegatesListCmd(flags))
	cmd.AddCommand(newGmailDelegatesGetCmd(flags))
	cmd.AddCommand(newGmailDelegatesAddCmd(flags))
	cmd.AddCommand(newGmailDelegatesRemoveCmd(flags))
	return cmd
}

func newGmailDelegatesListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all delegates",
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

			resp, err := svc.Users.Settings.Delegates.List("me").Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"delegates": resp.Delegates})
			}

			if len(resp.Delegates) == 0 {
				u.Err().Println("No delegates")
				return nil
			}

			tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "EMAIL\tSTATUS")
			for _, d := range resp.Delegates {
				fmt.Fprintf(tw, "%s\t%s\n",
					d.DelegateEmail,
					d.VerificationStatus)
			}
			_ = tw.Flush()
			return nil
		},
	}
}

func newGmailDelegatesGetCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get <delegateEmail>",
		Short: "Get a specific delegate's information",
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

			delegateEmail := args[0]
			delegate, err := svc.Users.Settings.Delegates.Get("me", delegateEmail).Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"delegate": delegate})
			}

			u.Out().Printf("delegate_email\t%s", delegate.DelegateEmail)
			u.Out().Printf("verification_status\t%s", delegate.VerificationStatus)
			return nil
		},
	}
}

func newGmailDelegatesAddCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "add <delegateEmail>",
		Short: "Add a delegate",
		Long: `Add a delegate to your mailbox.

The delegate will receive an email invitation that they must accept.
Once accepted, they can read, send, and delete messages on your behalf.

Note: This is a G Suite/Workspace feature and may not be available for personal Gmail accounts.`,
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

			delegateEmail := args[0]
			delegate := &gmail.Delegate{
				DelegateEmail: delegateEmail,
			}

			created, err := svc.Users.Settings.Delegates.Create("me", delegate).Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"delegate": created})
			}

			u.Out().Println("Delegate added successfully")
			u.Out().Printf("delegate_email\t%s", created.DelegateEmail)
			u.Out().Printf("verification_status\t%s", created.VerificationStatus)
			u.Out().Println("\nThe delegate will receive an invitation email that they must accept.")
			return nil
		},
	}
}

func newGmailDelegatesRemoveCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <delegateEmail>",
		Short: "Remove a delegate",
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

			delegateEmail := args[0]
			err = svc.Users.Settings.Delegates.Delete("me", delegateEmail).Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{
					"success":       true,
					"delegateEmail": delegateEmail,
				})
			}

			u.Out().Printf("Delegate %s removed successfully", delegateEmail)
			return nil
		},
	}
}
