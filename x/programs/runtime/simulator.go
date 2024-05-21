// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package simulator

import (
	"context"
	"fmt"
	"os"
)

func main() {
	if err := runSimulator(); err != nil {
		if _, err := fmt.Fprintln(os.Stderr, err); err != nil {
			panic(err)
		}
		os.Exit(1)
	}
	os.Exit(0)
}

func runSimulator() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	return (&Simulator{}).Execute(ctx)
}
