package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	lconsts "github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/consts"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/storage"
	"github.com/ava-labs/hypersdk/state"
)

var _ chain.Action = (*CreateToken)(nil)

type CreateToken struct {
	Name []byte `json:"name"`
	Symbol   []byte `json:"symbol"`
	Decimals uint8  `json:"decimals"`
	Metadata []byte `json:"metadata"`
}

// ComputeUnits implements chain.Action.
func (c *CreateToken) ComputeUnits(chain.Rules) uint64 {
	return CreateTokenComputeUnits
}

// Execute implements chain.Action.
// Returns: address of created token
func (c *CreateToken) Execute(ctx context.Context, r chain.Rules, mu state.Mutable, timestamp int64, actor codec.Address, actionID ids.ID) ([][]byte, error) {
	// Enforce initial invariants
	if len(c.Name) == 0 {
		return nil, ErrOutputTokenNameEmpty
	}
	if len(c.Symbol) == 0 {
		return nil, ErrOutputTokenSymbolEmpty
	}
	if len(c.Metadata) == 0 {
		return nil, ErrOutputTokenMetadataEmpty
	}

	if len(c.Name) > storage.MaxTokenNameSize {
		return nil, ErrOutputTokenNameTooLarge
	}
	if len(c.Symbol) > storage.MaxTokenSymbolSize {
		return nil, ErrOutputTokenSymbolTooLarge
	}
	if len(c.Metadata) > storage.MaxTokenMetadataSize {
		return nil, ErrOutputTokenMetadataTooLarge
	}

	if c.Decimals == 0 {
		return nil, ErrOutputTokenDecimalsZero
	}
	if c.Decimals > storage.MaxTokenDecimals {
		return nil, ErrOutputTokenDecimalsTooPrecise
	}
	// Continue only if address doesn't exist
	tokenAddress := storage.TokenAddress(c.Name, c.Symbol, c.Decimals, c.Metadata)
	tokenInfoKey := storage.TokenInfoKey(tokenAddress)
	
	if _, err := mu.GetValue(ctx, tokenInfoKey); err == nil {
		return nil, ErrOutputTokenAlreadyExists
	}
	
	// Invariants met; create and return
	if err := storage.SetTokenInfo(ctx, mu, tokenAddress, c.Name, c.Symbol, c.Decimals, c.Metadata, 0, actor); err != nil {
		return nil, err
	}

	// Return address
	return [][]byte{tokenAddress[:]}, nil
}

// Size implements chain.Action.
func (c *CreateToken) Size() int {
	return codec.BytesLen(c.Name) + codec.BytesLen(c.Symbol) + lconsts.Uint8Len + codec.BytesLen(c.Metadata)
}

// ValidRange implements chain.Action.
func (c *CreateToken) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

func (*CreateToken) GetTypeID() uint8 {
	return consts.CreateTokenID
}

func (c *CreateToken) StateKeys(_ codec.Address, actionID ids.ID) state.Keys {
	return state.Keys{
		string(storage.TokenInfoKey(storage.TokenAddress(c.Name, c.Symbol, c.Decimals, c.Metadata))): state.All,
	}
}

func (*CreateToken) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.TokenInfoChunks}
}

// Marshal implements chain.Action.
func (c *CreateToken) Marshal(p *codec.Packer) {
	p.PackBytes(c.Name)
	p.PackBytes(c.Symbol)
	p.PackByte(c.Decimals)
	p.PackBytes(c.Metadata)
}

func UnmarhsalCreateToken(p *codec.Packer) (chain.Action, error) {
	var createToken CreateToken
	p.UnpackBytes(storage.MaxTokenNameSize, true, &createToken.Name)
	p.UnpackBytes(storage.MaxTokenSymbolSize, true, &createToken.Symbol)
	createToken.Decimals = p.UnpackByte()
	p.UnpackBytes(storage.MaxTokenMetadataSize, true, &createToken.Metadata)
	return &createToken, p.Err()
}