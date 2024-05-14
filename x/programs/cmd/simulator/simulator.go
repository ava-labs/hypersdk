// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/cmd"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := &cmd.Simulator{}
	// initialize simulator vm
	err := s.Execute(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}
