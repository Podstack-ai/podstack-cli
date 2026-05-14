package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/Podstack-ai/podstack-cli/internal/relay"
	"github.com/Podstack-ai/podstack-cli/internal/transfer"
	"github.com/spf13/cobra"
)

type receiveFlags struct {
	out          string
	relay        string
	relayDefault bool
	yes          bool
}

func newReceiveCmd() *cobra.Command {
	flags := &receiveFlags{}

	cmd := &cobra.Command{
		Use:   "receive <code>",
		Short: "Receive files using a code phrase",
		Long: `Receive files using a code phrase shared by the sender.

Examples:
  podstack receive my-shared-code
  podstack receive my-shared-code --out ./downloads
  podstack receive --yes my-shared-code
  podstack receive --relay-default my-shared-code

Resume: if an earlier receive was interrupted, re-run the same command
in the same output directory and croc will resume the partial file
based on its hash.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			relayAddr, err := relay.Resolve(flags.relay, flags.relayDefault, strings.TrimSpace(os.Getenv("PODSTACK_RELAY")))
			if err != nil {
				return err
			}
			code := args[0]

			cfg := transfer.ReceiveConfig{
				Code:       code,
				Relay:      relayAddr,
				OutDir:     flags.out,
				AutoAccept: flags.yes,
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Relay: %s\nCode:  %s\n\n", relayAddr, code)

			return transfer.Receive(cfg)
		},
	}

	cmd.Flags().StringVar(&flags.out, "out", "", "output directory (default: cwd)")
	cmd.Flags().StringVar(&flags.relay, "relay", "", "relay host[:port] (overrides default)")
	cmd.Flags().BoolVar(&flags.relayDefault, "relay-default", false, "use croc's public relay (croc.schollz.com)")
	cmd.Flags().BoolVar(&flags.yes, "yes", false, "auto-accept the incoming transfer")

	return cmd
}
