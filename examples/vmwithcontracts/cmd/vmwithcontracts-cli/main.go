// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// "vmwithcontracts-cli" implements vmwithcontracts client operation interface.
package main

import (
	"os"

	"github.com/ava-labs/hypersdk/examples/vmwithcontracts/cmd/vmwithcontracts-cli/cmd"
	"github.com/ava-labs/hypersdk/utils"
)

func main() {
	if err := cmd.Execute(); err != nil {
		utils.Outf("{{red}}vmwithcontracts-cli exited with error:{{/}} %+v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
