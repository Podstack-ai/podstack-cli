package main

import (
	"fmt"
	"os"

	"github.com/podstack/podstack-cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "podstack:", err)
		os.Exit(1)
	}
}
