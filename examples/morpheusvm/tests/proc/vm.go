package proc

import (
	"encoding/json"

	"github.com/ava-labs/hypersdk/genesis"
)

type VM struct {
	// every VM has a genesis. We allows the VM to be created with a default genesis, or you can customize it yourself
	genesis *genesis.DefaultGenesis
	// pre-set keys on the vm
	Keys []*Ed25519TestKey
	// expectedABI

}

func NewVM() *VM {
	keys := newDefualtKeys()
	genesis := newDefaultGenesis(keys)
	return &VM{
		genesis: genesis,
		Keys:    keys,
	}
}

func (v *VM)GetGenesisBytes() ([]byte, error) {
	return json.Marshal(v.genesis)
}

