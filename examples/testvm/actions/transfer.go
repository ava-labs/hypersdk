package actions

import (
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/testvm/consts"
	"github.com/ava-labs/hypersdk/vm/components"
	"github.com/ava-labs/hypersdk/vm/components/token"
)

func NewNativeTransfer(
	to codec.Address,
	value uint64,
	memo []byte,
) *components.GenericAction {
	transfer := &token.Transfer{
		Name: consts.Symbol,
		To: to,
		Value: value,
		Memo: memo,
	}

	transferAction := components.NewGenericAction(
		transfer,
		consts.TransferID,
		-1,
		1,
	)

	return transferAction
}

func NewTransferAction() *components.GenericAction  {
	transferAction := components.NewGenericAction(
		&token.Transfer{},
		consts.TransferID,
		-1,
		1,
	)

	return transferAction
}

func NewTransferResult() *components.GenericType {
	transferResult := components.NewGenericType(
		&token.TransferResult{},
		consts.TransferID,
	)

	return transferResult
}