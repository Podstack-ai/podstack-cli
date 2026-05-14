package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "podstack",
	Short: "Send and receive large files using croc",
	Long: `podstack is a thin wrapper around croc (https://github.com/schollz/croc).

It defaults to croc's public relay (croc.schollz.com) and supports
sending files, directories, and text. Interrupted transfers resume
automatically when the receiver is re-run with the same code in the same
output directory.`,
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

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(newSendCmd())
	rootCmd.AddCommand(newReceiveCmd())
}

func Execute() error {
	return rootCmd.Execute()
}
