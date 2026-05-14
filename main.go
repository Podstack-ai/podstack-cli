package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/Podstack-ai/podstack-cli/cmd"
	"github.com/Podstack-ai/podstack-cli/internal/relay"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "podstack:", err)
		if errors.Is(err, relay.ErrConflictingFlags) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}
