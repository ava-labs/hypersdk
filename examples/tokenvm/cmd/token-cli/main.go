// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// "token-cli" implements tokenvm client operation interface.
package main

import (
	"os"

	"github.com/fatih/color"

	"github.com/ava-labs/hypersdk/examples/tokenvm/cmd/token-cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		color.Red("token-cli failed: %v", err)
		os.Exit(1)
	}
	os.Exit(0)
}
