// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/hypersdk/vm"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

const Namespace = "controller"

func With() vm.Option {
	return vm.NewOption(Namespace, func(v *vm.VM, _ []byte) error {
		vm.WithVMAPIs(jsonRPCServerFactory{})(v)
		return nil
	})
}

func WithRuntime() vm.Option {
	return vm.NewOption(Namespace+"runtime", func(v *vm.VM, _ []byte) error {
		wasmRuntime = runtime.NewRuntime(runtime.NewConfig(), v.Logger())
		return nil
	})
}
