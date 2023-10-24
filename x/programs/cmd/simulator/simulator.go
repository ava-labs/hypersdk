// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"context"
	"os"

	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/cmd"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := cmd.NewRootCmd().ExecuteContext(ctx); err != nil {
		utils.Outf("{{red}}error: {{/}}%+v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
