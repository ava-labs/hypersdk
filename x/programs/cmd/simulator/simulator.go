package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/cmd"
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

	return (&cmd.Simulator{}).Execute(ctx)
}
