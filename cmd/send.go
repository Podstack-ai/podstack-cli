package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/podstack/podstack-cli/internal/relay"
	"github.com/podstack/podstack-cli/internal/transfer"
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
}

func newSendCmd() *cobra.Command {
	flags := &sendFlags{}

	cmd := &cobra.Command{
		Use:   "send [files-or-dirs...]",
		Short: "Send files, directories, or text",
		Long: `Send files, directories, or text via croc.

Examples:
  podstack send ./episode-001.wav
  podstack send ./assets/ ./notes.md
  podstack send --text "see you at 3pm"
  podstack send --code my-shared-code ./file.zip
  podstack send --relay-default ./file.zip       # use croc's public relay
  podstack send --relay myrelay.example.com:9009 ./file.zip

Resume: if a send is interrupted, re-run the same command and the
receiver will resume from where it left off (croc tracks partial files
by hash in the receive directory).`,
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
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Relay: %s\nCode:  %s\n\n", relayAddr, code)
			fmt.Fprintln(cmd.OutOrStdout(), "Receiver runs: podstack receive", code)
			fmt.Fprintln(cmd.OutOrStdout())

			return transfer.Send(cfg)
		},
	}

	cmd.Flags().StringVar(&flags.code, "code", "", "custom code phrase (≥6 chars; auto-generated if empty)")
	cmd.Flags().StringVar(&flags.relay, "relay", "", "relay host[:port] (overrides default)")
	cmd.Flags().BoolVar(&flags.relayDefault, "relay-default", false, "use croc's public relay (croc.schollz.com)")
	cmd.Flags().StringVar(&flags.text, "text", "", "send text instead of a file")
	cmd.Flags().BoolVar(&flags.zip, "zip", false, "zip directories before sending")
	cmd.Flags().BoolVar(&flags.noCompress, "no-compress", false, "disable compression")

	return cmd
}
