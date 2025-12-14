package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/steipete/gogcli/internal/outfmt"
	"github.com/steipete/gogcli/internal/ui"
	"google.golang.org/api/gmail/v1"
)

func newGmailSendAsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sendas",
		Short: "Manage send-as aliases (send email from different addresses)",
	}

	cmd.AddCommand(newGmailSendAsListCmd(flags))
	cmd.AddCommand(newGmailSendAsGetCmd(flags))
	cmd.AddCommand(newGmailSendAsCreateCmd(flags))
	cmd.AddCommand(newGmailSendAsVerifyCmd(flags))
	cmd.AddCommand(newGmailSendAsDeleteCmd(flags))
	cmd.AddCommand(newGmailSendAsUpdateCmd(flags))
	return cmd
}

func newGmailSendAsListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List send-as aliases",
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

			resp, err := svc.Users.Settings.SendAs.List("me").Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"sendAs": resp.SendAs})
			}

			if len(resp.SendAs) == 0 {
				u.Err().Println("No send-as aliases")
				return nil
			}

			tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "EMAIL\tDISPLAY NAME\tDEFAULT\tVERIFIED\tTREAT AS ALIAS")
			for _, sa := range resp.SendAs {
				isDefault := ""
				if sa.IsDefault {
					isDefault = "yes"
				}
				verified := "pending"
				if sa.VerificationStatus == "accepted" {
					verified = "yes"
				}
				treatAsAlias := ""
				if sa.TreatAsAlias {
					treatAsAlias = "yes"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
					sa.SendAsEmail, sa.DisplayName, isDefault, verified, treatAsAlias)
			}
			_ = tw.Flush()
			return nil
		},
	}
}

func newGmailSendAsGetCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get <email>",
		Short: "Get details of a send-as alias",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			sendAsEmail := strings.TrimSpace(args[0])
			if sendAsEmail == "" {
				return errors.New("email is required")
			}

			svc, err := newGmailService(cmd.Context(), account)
			if err != nil {
				return err
			}

			sa, err := svc.Users.Settings.SendAs.Get("me", sendAsEmail).Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"sendAs": sa})
			}

			u.Out().Printf("send_as_email\t%s", sa.SendAsEmail)
			u.Out().Printf("display_name\t%s", sa.DisplayName)
			u.Out().Printf("reply_to\t%s", sa.ReplyToAddress)
			u.Out().Printf("signature\t%s", sa.Signature)
			u.Out().Printf("is_primary\t%t", sa.IsPrimary)
			u.Out().Printf("is_default\t%t", sa.IsDefault)
			u.Out().Printf("treat_as_alias\t%t", sa.TreatAsAlias)
			u.Out().Printf("verification_status\t%s", sa.VerificationStatus)
			return nil
		},
	}
}

func newGmailSendAsCreateCmd(flags *rootFlags) *cobra.Command {
	var displayName string
	var replyTo string
	var signature string
	var treatAsAlias bool

	cmd := &cobra.Command{
		Use:   "create <email>",
		Short: "Create a new send-as alias",
		Long: `Create a new send-as alias. After creation, a verification email will be sent
to the specified address. The alias cannot be used until verified.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			sendAsEmail := strings.TrimSpace(args[0])
			if sendAsEmail == "" {
				return errors.New("email is required")
			}

			svc, err := newGmailService(cmd.Context(), account)
			if err != nil {
				return err
			}

			sendAs := &gmail.SendAs{
				SendAsEmail:    sendAsEmail,
				DisplayName:    displayName,
				ReplyToAddress: replyTo,
				Signature:      signature,
				TreatAsAlias:   treatAsAlias,
			}

			created, err := svc.Users.Settings.SendAs.Create("me", sendAs).Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"sendAs": created})
			}

			u.Out().Printf("send_as_email\t%s", created.SendAsEmail)
			u.Out().Printf("verification_status\t%s", created.VerificationStatus)
			u.Err().Println("Verification email sent. Check your inbox to complete setup.")
			return nil
		},
	}

	cmd.Flags().StringVar(&displayName, "display-name", "", "Name that appears in the From field")
	cmd.Flags().StringVar(&replyTo, "reply-to", "", "Reply-to address (optional)")
	cmd.Flags().StringVar(&signature, "signature", "", "HTML signature for emails sent from this alias")
	cmd.Flags().BoolVar(&treatAsAlias, "treat-as-alias", true, "Treat as alias (replies sent from Gmail web)")
	return cmd
}

func newGmailSendAsVerifyCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "verify <email>",
		Short: "Resend verification email for a send-as alias",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			sendAsEmail := strings.TrimSpace(args[0])
			if sendAsEmail == "" {
				return errors.New("email is required")
			}

			svc, err := newGmailService(cmd.Context(), account)
			if err != nil {
				return err
			}

			err = svc.Users.Settings.SendAs.Verify("me", sendAsEmail).Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{
					"email":   sendAsEmail,
					"message": "Verification email sent",
				})
			}

			u.Out().Printf("Verification email sent to %s", sendAsEmail)
			return nil
		},
	}
}

func newGmailSendAsDeleteCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <email>",
		Short: "Delete a send-as alias",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			sendAsEmail := strings.TrimSpace(args[0])
			if sendAsEmail == "" {
				return errors.New("email is required")
			}

			svc, err := newGmailService(cmd.Context(), account)
			if err != nil {
				return err
			}

			err = svc.Users.Settings.SendAs.Delete("me", sendAsEmail).Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{
					"email":   sendAsEmail,
					"deleted": true,
				})
			}

			u.Out().Printf("Deleted send-as alias: %s", sendAsEmail)
			return nil
		},
	}
}

func newGmailSendAsUpdateCmd(flags *rootFlags) *cobra.Command {
	var displayName string
	var replyTo string
	var signature string
	var treatAsAlias bool
	var makeDefault bool

	cmd := &cobra.Command{
		Use:   "update <email>",
		Short: "Update a send-as alias",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := ui.FromContext(cmd.Context())
			account, err := requireAccount(flags)
			if err != nil {
				return err
			}
			sendAsEmail := strings.TrimSpace(args[0])
			if sendAsEmail == "" {
				return errors.New("email is required")
			}

			svc, err := newGmailService(cmd.Context(), account)
			if err != nil {
				return err
			}

			// Get current settings first
			current, err := svc.Users.Settings.SendAs.Get("me", sendAsEmail).Do()
			if err != nil {
				return err
			}

			// Update only provided fields
			if cmd.Flags().Changed("display-name") {
				current.DisplayName = displayName
			}
			if cmd.Flags().Changed("reply-to") {
				current.ReplyToAddress = replyTo
			}
			if cmd.Flags().Changed("signature") {
				current.Signature = signature
			}
			if cmd.Flags().Changed("treat-as-alias") {
				current.TreatAsAlias = treatAsAlias
			}
			if cmd.Flags().Changed("make-default") {
				current.IsDefault = makeDefault
			}

			updated, err := svc.Users.Settings.SendAs.Update("me", sendAsEmail, current).Do()
			if err != nil {
				return err
			}

			if outfmt.IsJSON(cmd.Context()) {
				return outfmt.WriteJSON(os.Stdout, map[string]any{"sendAs": updated})
			}

			u.Out().Printf("Updated send-as alias: %s", updated.SendAsEmail)
			return nil
		},
	}

	cmd.Flags().StringVar(&displayName, "display-name", "", "Name that appears in the From field")
	cmd.Flags().StringVar(&replyTo, "reply-to", "", "Reply-to address")
	cmd.Flags().StringVar(&signature, "signature", "", "HTML signature")
	cmd.Flags().BoolVar(&treatAsAlias, "treat-as-alias", true, "Treat as alias")
	cmd.Flags().BoolVar(&makeDefault, "make-default", false, "Make this the default send-as address")
	return cmd
}
