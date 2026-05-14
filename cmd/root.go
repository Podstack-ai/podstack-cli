package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

var version = "dev"

// Banner is the Podstack.ai ASCII banner shown on `podstack --help` and
// before each send/receive operation. ASCII-only so it renders on every
// terminal podstack ships to.
const Banner = `
  ____           _     _             _              _
 |  _ \ ___   __| |___| |_ __ _  ___| | __     __ _(_)
 | |_) / _ \ / _` + "`" + ` / __| __/ _` + "`" + ` |/ __| |/ /  _ / _` + "`" + ` | |
 |  __/ (_) | (_| \__ \ || (_| | (__|   <  |_| | (_| | |
 |_|   \___/ \__,_|___/\__\__,_|\___|_|\_\     \__,_|_|

      Podstack.ai CLI -- the ML cloud, command line.`

var rootCmd = &cobra.Command{
	Use:   "podstack",
	Short: "Podstack.ai CLI",
	Long: Banner + `

podstack is the official command-line interface for podstack.ai.

This release ships peer-to-peer file transfer for moving model weights,
datasets, and other large artifacts between machines. More podstack.ai
features (model deployment, GPU lease management, run logs, billing) will
land in future releases.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the podstack version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version)
	},
}

// PrintBanner writes the Podstack banner to w. Commands print it at the
// start of a session so users see Podstack branding regardless of which
// subcommand they invoked.
func PrintBanner(w io.Writer) {
	fmt.Fprintln(w, Banner)
	fmt.Fprintln(w)
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(newSendCmd())
	rootCmd.AddCommand(newReceiveCmd())
}

func Execute() error {
	return rootCmd.Execute()
}
