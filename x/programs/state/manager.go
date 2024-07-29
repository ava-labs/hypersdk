package state

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

// ensure SimulatorStateManager implements StateManager
var _ runtime.StateManager = &programStateManager{}

type programStateManager struct {
   state *SimulatorState
}

// Balance Manager Methods
func (p *programStateManager) GetBalance(ctx context.Context, address codec.Address) (uint64, error) {
   return 0, nil
}

func (p *programStateManager) TransferBalance(ctx context.Context, from codec.Address, to codec.Address, amount uint64) error {
   return nil
}

// ProgramManager methods
func (p *programStateManager) GetProgramState(address codec.Address) state.Mutable {
   return p.state
}

func (p *programStateManager) GetAccountProgram(ctx context.Context, account codec.Address) (ids.ID, error) {
	   return ids.Empty, nil
}

func (p *programStateManager) GetProgramBytes(ctx context.Context, programID ids.ID) ([]byte, error) {
   return nil, nil
}

func (p *programStateManager) NewAccountWithProgram(ctx context.Context, programID ids.ID, accountCreationData []byte) (codec.Address, error) {
   return codec.EmptyAddress, nil
}

func (p *programStateManager) SetAccountProgram(ctx context.Context, account codec.Address, programID ids.ID) error {
   return nil
}