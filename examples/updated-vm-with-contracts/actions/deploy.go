package actions

import (
	"context"
	"crypto/sha256"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/updated-vm-with-contracts/consts"
	"github.com/ava-labs/hypersdk/examples/updated-vm-with-contracts/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/contracts/runtime"
)

var _ chain.Action = (*Deploy)(nil)

// Deploy, deploys a contract to the chain with the given bytes
type Deploy struct {
	// ContractBytes is the wasm bytes of the contract being deployed
	ContractBytes []byte `json:"contractBytes"`
	// Creation Data used for generating random accounts
	CreationData []byte `json:"creationData"`
}

// units to execute this action
func (*Deploy) ComputeUnits(chain.Rules) uint64 {
	// TODO: charge more if contract has not been deployed before(because adding to storage should be more expensive)
	return consts.DeployUnits
}

// Specify all statekeys Execute can touch
func (d *Deploy) StateKeys(actor codec.Address) state.Keys {
	contractID := sha256.Sum256(d.ContractBytes)
	contractBytesKey := storage.ContractBytesKey(contractID[:])
	account := storage.GetAccountAddress(contractID[:], d.CreationData)
	contractAccountKey := storage.AccountContractIDKey(account)

	return state.Keys{
		string(contractBytesKey):   state.Read | state.Write,
		string(contractAccountKey): state.Read | state.Write,
	}
}

// Execute deploys the contract to the chain, returning {deploy contract ID, deployed contract address}
func (d *Deploy) Execute(ctx context.Context, rules chain.Rules, mu state.Mutable, timestamp int64, actor codec.Address, actionID ids.ID) (codec.Typed, error) {
	contractStateManager := &storage.ContractStateManager{Mutable: mu}
	// gets the contract ID by hashing the contract bytes
	contractID := sha256.Sum256(d.ContractBytes)

	// no need to re-set the bytes
	if _, err := contractStateManager.GetContractBytes(ctx, contractID[:]); err == nil {
		account := storage.GetAccountAddress(contractID[:], d.CreationData)
		return &DeployOutput{contractID[:], account}, nil
	}

	// add the contract bytes to the storage
	contractStateManager.SetContractBytes(ctx, contractID[:], d.ContractBytes)

	// set the contract id
	account, err := contractStateManager.NewAccountWithContract(ctx, contractID[:], d.CreationData)
	if err != nil {
		return &DeployOutput{}, nil
	}

	return &DeployOutput{contractID[:], account}, nil
}

// Object interface
func (*Deploy) GetTypeID() uint8 {
	return consts.DeployID
}

func (*Deploy) ValidRange(rules chain.Rules) (int64, int64) {
	return -1, -1
}

type DeployOutput struct {
	ID      runtime.ContractID
	Account codec.Address
}

func (*DeployOutput) GetTypeID() uint8 {
	return consts.DeployOutputId
}

var _ chain.Marshaler = (*Deploy)(nil)

func (d *Deploy) Size() int {
	return len(d.ContractBytes) + len(d.CreationData)
}

func (d *Deploy) Marshal(p *codec.Packer) {
	p.PackBytes(d.ContractBytes)
	p.PackBytes(d.CreationData)
}

func UnmarshalDeploy(p *codec.Packer) (chain.Action, error) {
	var deployContract Deploy
	contractBytes := p.Packer.UnpackBytes()
	contractCreationData := p.Packer.UnpackBytes()
	deployContract.ContractBytes = contractBytes
	deployContract.CreationData = contractCreationData
	return &deployContract, nil
}
