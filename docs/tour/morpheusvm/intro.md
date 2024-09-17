# [Fake] Tour of HyperSDK

In this series, we will move away from exploring VMs in its entirety to instead
exploring the various components that we believe are important. In particular,
we'll go over the following components:

- Execute
- State Keys

To get started, make sure you are in the `examples/` directory of HyperSDK. Then
run the following command:

```bash
cp -r ./morpheusvm/ ./morpheusvmtour
```

This will create a copy of MorpheusVM and put it in the `morpheusvmtour/`
subdirectory. Afterwards, run the following:

```bash
rm ./morpheusvmtour/actions/transfer.go
touch ./morpheusvmtour/actions/transfer.go
```

Finally, paste the following into `transfer.go`:

```golang
package actions

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/state"

	consts "github.com/ava-labs/hypersdk/consts"
	mconsts "github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
)

const (
	TransferComputeUnits = 1
	MaxMemoSize          = 256
)

var (
	ErrOutputValueZero                 = errors.New("value is zero")
	ErrOutputMemoTooLarge              = errors.New("memo is too large")
	_                     chain.Action = (*Transfer)(nil)
)

type Transfer struct {
	// To is the recipient of the [Value].
	To codec.Address `serialize:"true" json:"to"`

	// Amount are transferred to [To].
	Value uint64 `serialize:"true" json:"value"`

	// Optional message to accompany transaction.
	Memo codec.Bytes `serialize:"true" json:"memo"`
}

func (*Transfer) GetTypeID() uint8 {
	return mconsts.TransferID
}

func (t *Transfer) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	panic("unimplemented")
}

func (*Transfer) StateKeysMaxChunks() []uint16 {
	panic("unimplemented")
}

func (t *Transfer) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	actor codec.Address,
	_ ids.ID,
) ([][]byte, error) {
	panic("unimplemented")
}

func (*Transfer) ComputeUnits(chain.Rules) uint64 {
	return TransferComputeUnits
}

func (*Transfer) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

var _ chain.Marshaler = (*Transfer)(nil)

func (t *Transfer) Size() int {
	return codec.AddressLen + consts.Uint64Len + codec.BytesLen(t.Memo)
}

func (t *Transfer) Marshal(p *codec.Packer) {
	p.PackAddress(t.To)
	p.PackLong(t.Value)
	p.PackBytes(t.Memo)
}

func UnmarshalTransfer(p *codec.Packer) (chain.Action, error) {
	var transfer Transfer
	p.UnpackAddress(&transfer.To)
	transfer.Value = p.UnpackUint64(true)
	p.UnpackBytes(MaxMemoSize, false, (*[]byte)(&transfer.Memo))
	return &transfer, p.Err()
}
```

We'll focus on implementing `Execute()`, `StateKeys()`, and
`StateKeysMaxChunks()` in the following sections.
