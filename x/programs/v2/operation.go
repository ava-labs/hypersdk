// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package v2

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/near/borsh-go"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/cmd"
	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/actions"
	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/storage"
	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/utils"
)

type KeyAlgorithm string

const (
	KeyEd25519   KeyAlgorithm = "ed25519"
	KeySecp256k1 KeyAlgorithm = "secp256k1"
)

type Operation interface {
	Execute(ctx context.Context, log logging.Logger, db *state.SimpleMutable, resp *cmd.Response) error
}

var _ Operation = (*Key)(nil)

type Key struct {
	Name      string;
	Algorithm KeyAlgorithm;
}

// TODO better package management ! This function was already defined
func keyCreateFunc(ctx context.Context, db *state.SimpleMutable, name string) (ed25519.PublicKey, error) {
	priv, err := ed25519.GeneratePrivateKey()
	if err != nil {
		return ed25519.EmptyPublicKey, err
	}
	ok, err := hasKey(ctx, db, name)
	if ok {
		return ed25519.EmptyPublicKey, fmt.Errorf("%w: %s", cmd.ErrDuplicateKeyName, name)
	}
	if err != nil {
		return ed25519.EmptyPublicKey, err
	}
	err = storage.SetKey(ctx, db, priv, name)
	if err != nil {
		return ed25519.EmptyPublicKey, err
	}

	err = db.Commit(ctx)
	if err != nil {
		return ed25519.EmptyPublicKey, err
	}

	return priv.PublicKey(), nil
}

func hasKey(ctx context.Context, db state.Immutable, name string) (bool, error) {
	_, ok, err := storage.GetPublicKey(ctx, db, name)
	return ok, err
}

func (k *Key) Execute(ctx context.Context, log logging.Logger, db *state.SimpleMutable, resp *cmd.Response) error {
	key, err := keyCreateFunc(ctx, db, k.name)
	if errors.Is(err, cmd.ErrDuplicateKeyName) {
		log.Debug("key already exists")
	} else if err != nil {
		return err
	}

	resp.SetMsg(fmt.Sprintf("created named key with address %s", utils.Address(key)))
	return nil
}

var _ Operation = (*Call)(nil)

type Call struct {
	ReadOnly  bool				 `json:"readOnly"`
	ProgramID ids.ID 			 `json:"programID"`
	Method		string 			 `json:"string"`
	Data		  []byte 			 `json:"data,omitempty"`;
	MaxUnits  uint64 			 `json:"maxUnits"`;
	Require   *cmd.Require `json:"require,omitempty" yaml:"require,omitempty"`
}

// generateRandomID creates a unique ID.
// Note: ids.GenerateID() is not used because the IDs are not unique and will
// collide.
func generateRandomID() (ids.ID, error) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		return ids.Empty, err
	}
	id, err := ids.ToID(key)
	if err != nil {
		return ids.Empty, err
	}

	return id, nil
}

func programExecuteFunc(
	ctx context.Context,
	log logging.Logger,
	db *state.SimpleMutable,
	programID ids.ID,
	data []byte,
	function string,
	maxUnits uint64,
) (ids.ID, []byte, uint64, error) {
	// simulate create program transaction
	programTxID, err := generateRandomID()
	if err != nil {
		return ids.Empty, nil, 0, err
	}

	programExecuteAction := actions.ProgramExecute{
		Function: function,
		MaxUnits: maxUnits,
		ProgramID: programID,
		Params:   data,
		Log:      log,
	}

	// execute the action
	success, _, resp, err := programExecuteAction.Execute(ctx, nil, db, 0, codec.EmptyAddress, programTxID)

	if !success {
		return ids.Empty, nil, 0, fmt.Errorf("program execution failed: %s", string(resp))
	}
	if err != nil {
		return ids.Empty, nil, 0, err
	}


	// store program to disk only on success
	err = db.Commit(ctx)
	if err != nil {
		return ids.Empty, nil, 0, err
	}

	// get remaining balance from runtime meter
	balance, err := programExecuteAction.GetBalance()

	return programTxID, resp, balance, err
}

// validateAssertion validates the assertion against the actual value.
func validateAssertion(bytes []byte, require *cmd.Require) (bool, error) {
	if require == nil {
		return true, nil
	}

	actual := int64(0)
	borsh.Deserialize(&actual, bytes)
	assertion := require.Result
	// convert the assertion value(string) to uint64
	value, err := strconv.ParseInt(assertion.Value, 10, 64)
	if err != nil {
		return false, err
	}
	// TODO REMOVE ME
	_ = value

	// TODO
	// switch Operator(assertion.Operator) {
	// case NumericGt:
	// 	if actual > value {
	// 		return true, nil
	// 	}
	// case NumericLt:
	// 	if actual < value {
	// 		return true, nil
	// 	}
	// case NumericGe:
	// 	if actual >= value {
	// 		return true, nil
	// 	}
	// case NumericLe:
	// 	if actual <= value {
	// 		return true, nil
	// 	}
	// case NumericEq:
	// 	if actual == value {
	// 		return true, nil
	// 	}
	// case NumericNe:
	// 	if actual != value {
	// 		return true, nil
	// 	}
	// default:
	// 	return false, fmt.Errorf("invalid assertion operator: %s", assertion.Operator)
	// }

	return false, nil
}

func (c *Call) Execute(ctx context.Context, log logging.Logger, db *state.SimpleMutable, resp *cmd.Response) error {
	maxUnits := c.MaxUnits
	if c.ReadOnly {
		maxUnits = math.MaxUint64
	}

	id, response, balance, err := programExecuteFunc(ctx, log, db, c.ProgramID, c.Data, c.Method, maxUnits)
	if err != nil {
		return err
	}
	resp.SetResponse(response)

	ok, err := validateAssertion(response, c.Require)

	if !ok {
		return fmt.Errorf("%w", cmd.ErrResultAssertionFailed)
	}
	if err != nil {
		return err
	}

	if !c.ReadOnly {
		resp.SetTxID(id.String())
		resp.SetBalance(balance)
	}

	return nil
}

var _ Operation = (*Create)(nil)

type Create struct {
	Path string;
}

// createProgram simulates a create program transaction and stores the program to disk.
func programCreateFunc(ctx context.Context, db *state.SimpleMutable, path string) (ids.ID, error) {
	programBytes, err := os.ReadFile(path)
	if err != nil {
		return ids.Empty, err
	}

	// simulate create program transaction
	programID, err := generateRandomID()
	if err != nil {
		return ids.Empty, err
	}

	programCreateAction := actions.ProgramCreate{
		Program: programBytes,
	}

	// execute the action
	success, _, output, err := programCreateAction.Execute(ctx, nil, db, 0, codec.EmptyAddress, programID)
	if output != nil {
		fmt.Println(string(output))
	}
	if !success {
		return ids.Empty, fmt.Errorf("program creation failed: %s", err)
	}
	if err != nil {
		return ids.Empty, err
	}

	// store program to disk only on success
	err = db.Commit(ctx)
	if err != nil {
		return ids.Empty, err
	}

	return programID, nil
}

func (c *Create) Execute(ctx context.Context, log logging.Logger, db *state.SimpleMutable, resp *cmd.Response) error {
	// get program path from params
	id, err := programCreateFunc(ctx, db, c.path)
	if err != nil {
		return err
	}
	resp.SetTxID(id.String())
	resp.SetTimestamp(time.Now().Unix())

	return nil
}

