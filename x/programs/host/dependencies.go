// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package host

import "github.com/ava-labs/hypersdk/x/programs/program"

type Import interface {
	// Name returns the name of this import module.
	Name() string
	// Register registers this import module with the provided link.
	Register(*Link, *program.Context) error
}
