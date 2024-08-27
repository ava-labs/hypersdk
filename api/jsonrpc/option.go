// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package jsonrpc

import "github.com/ava-labs/hypersdk/vm"

const Namespace = "core"

func With() vm.Option {
	return vm.NewOption(Namespace, func(v *vm.VM, _ []byte) error {
		vm.WithVMAPIs(JSONRPCServerFactory{})(v)
		return nil
	})
}
