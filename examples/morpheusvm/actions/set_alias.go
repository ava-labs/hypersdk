// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	mconsts "github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
)

var _ chain.Action = (*SetAlias)(nil)

type SetAlias struct {
	// Alias is the alias to set
	Alias string `json:"alias"`

	// Address is the address to set the alias to
	Address codec.Address `json:"address"`
}

func (*SetAlias) GetTypeID() uint8 {
	return mconsts.SetAliasID
}

func (t *SetAlias) StateKeys(auth chain.Auth, _ ids.ID) []string {
	return []string{
		string(storage.AliasKey(t.Alias)),
	}
}

func (*SetAlias) OutputsWarpMessage() bool {
	return false
}

func (s *SetAlias) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	auth chain.Auth,
	_ ids.ID,
	_ bool,
) (bool, uint64, []byte, *warp.UnsignedMessage, error) {
	if s.Alias == "" {
		return false, SetAliasComputeUnits, AliasEmpty, nil, nil
	}

	if len(s.Alias) != AliasLength {
		return false, SetAliasComputeUnits, AliasLengthInvalid, nil, nil
	}

	if err := storage.SetAlias(ctx, mu, s.Alias, s.Address); err != nil {
		return false, SetAliasComputeUnits, utils.ErrBytes(err), nil, nil
	}

	return true, SetAliasComputeUnits, nil, nil, nil
}

func (s *SetAlias) MaxComputeUnits(chain.Rules) uint64 {
	return SetAliasComputeUnits
}

func (s *SetAlias) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.AliasChunks}
}

func (s *SetAlias) Size() int {
	return codec.StringLen(s.Alias) + codec.AddressLen
}

func (s *SetAlias) Marshal(p *codec.Packer) {
	p.PackString(s.Alias)
	p.PackAddress(s.Address)
}

func UnmarshalSetAlias(p *codec.Packer, _ *warp.Message) (chain.Action, error) {
	var setAlias SetAlias
	setAlias.Alias = p.UnpackString(true)
	p.UnpackAddress(&setAlias.Address)

	if err := p.Err(); err != nil {
		return nil, err
	}

	return &setAlias, nil
}

func (*SetAlias) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}
