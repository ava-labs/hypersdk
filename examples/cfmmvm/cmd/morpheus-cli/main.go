// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// "morpheus-cli" implements cfmmvm client operation interface.
package main

import (
	"os"

	"github.com/ava-labs/hypersdk/examples/cfmmvm/cmd/morpheus-cli/cmd"
	"github.com/ava-labs/hypersdk/utils"
)

func main() {
	if err := cmd.Execute(); err != nil {
		utils.Outf("{{red}}morpheus-cli exited with error:{{/}} %+v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
