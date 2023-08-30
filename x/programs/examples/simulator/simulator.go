// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"os"

	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/x/programs/examples/simulator/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		utils.Outf("{{red}}hypersdk program simulator exited with error:{{/}} %+v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
