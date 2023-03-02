// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms"

	"github.com/ava-labs/hypersdk/examples/tokenvm/controller"
)

var _ vms.Factory = &Factory{}

type Factory struct{}

func (*Factory) New(logging.Logger) (interface{}, error) {
	return controller.New(), nil
}
