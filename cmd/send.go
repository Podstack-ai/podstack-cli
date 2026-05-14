package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/Podstack-ai/podstack-cli/internal/relay"
	"github.com/Podstack-ai/podstack-cli/internal/transfer"
	crocutils "github.com/schollz/croc/v10/src/utils"
	"github.com/spf13/cobra"
)

type sendFlags struct {
	code         string
	relay        string
	relayDefault bool
	text         string
	zip          bool
	noCompress   bool
	transfers    int
}

func newSendCmd() *cobra.Command {
	flags := &sendFlags{}

	cmd := &cobra.Command{
		Use:   "send [files-or-dirs...]",
		Short: "Send files, directories, or text to another machine",
		Long: `Send files, directories, or text to another machine.

Examples:
  podstack send ./model.bin
  podstack send ./checkpoints/ ./config.yaml
  podstack send --text "training finished, model.bin coming next"
  podstack send --code my-shared-code ./big.zip
  podstack send --transfers 8 ./huge-dataset.tar

Resume: if a send is interrupted, re-run the same command. The receiver
will pick up where it left off based on the partial file's hash.

Throughput: transfers are split across N parallel TCP streams (default 4).
Bump --transfers for large files on high-bandwidth links.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			relayAddr, err := relay.Resolve(flags.relay, flags.relayDefault, strings.TrimSpace(os.Getenv("PODSTACK_RELAY")))
			if err != nil {
				return err
			}

			code := flags.code
			if code == "" {
				code = crocutils.GetRandomName()
			}

			cfg := transfer.SendConfig{
				Code:       code,
				Relay:      relayAddr,
				Paths:      args,
				Text:       flags.text,
				ZipFolder:  flags.zip,
				NoCompress: flags.noCompress,
				Transfers:  flags.transfers,
			}

			PrintBanner(cmd.OutOrStdout())
			fmt.Fprintf(cmd.OutOrStdout(), "Relay:  %s\nCode:   %s\n\n", relayAddr, code)
			fmt.Fprintln(cmd.OutOrStdout(), "Receiver runs: podstack receive", code)
			fmt.Fprintln(cmd.OutOrStdout())

			return transfer.Send(cfg)
		},
	}

	cmd.Flags().StringVar(&flags.code, "code", "", "custom code phrase (>=6 chars; auto-generated if empty)")
	cmd.Flags().StringVar(&flags.relay, "relay", "", "relay host[:port] (overrides default)")
	cmd.Flags().BoolVar(&flags.relayDefault, "relay-default", false, "use the upstream public relay instead of the Podstack one")
	cmd.Flags().StringVar(&flags.text, "text", "", "send text instead of a file")
	cmd.Flags().BoolVar(&flags.zip, "zip", false, "zip directories before sending")
	cmd.Flags().BoolVar(&flags.noCompress, "no-compress", false, "disable compression")
	cmd.Flags().IntVar(&flags.transfers, "transfers", transfer.DefaultTransfers, "number of parallel TCP streams")

	return cmd
}
