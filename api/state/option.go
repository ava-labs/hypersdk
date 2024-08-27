// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import "github.com/ava-labs/hypersdk/vm"

const Namespace = "corestate"

func With() vm.Option {
	return vm.NewOption(Namespace, func(v *vm.VM, _ []byte) error {
		vm.WithVMAPIs(JSONRPCStateServerFactory{})(v)
		return nil
	})
}
