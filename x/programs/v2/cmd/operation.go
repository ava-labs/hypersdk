// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/v2/vm/actions"
	"github.com/ava-labs/hypersdk/x/programs/v2/vm/utils"
)

type KeyAlgorithm string

const (
	KeyEd25519   KeyAlgorithm = "ed25519"
	KeySecp256k1 KeyAlgorithm = "secp256k1"
)

type Operation interface {
	Execute(ctx context.Context, log logging.Logger, db *state.SimpleMutable, resp *Response) error
}

var _ Operation = (*KeyStep)(nil)

type KeyStep struct {
	Name      string			`json:"name"`
	Curve KeyAlgorithm `json:"curve"`
}

func (k *KeyStep) Execute(ctx context.Context, log logging.Logger, db *state.SimpleMutable, resp *Response) error {
	// TODO validate the algorithm
	key, err := keyCreateFunc(ctx, db, k.Name)
	if errors.Is(err, ErrDuplicateKeyName) {
		log.Debug("key already exists")
	} else if err != nil {
		return err
	}

	resp.SetMsg(fmt.Sprintf("created named key with address %s", utils.Address(key)))
	return nil
}

var _ Operation = (*CallStep)(nil)

type CallStep struct {
	ReadOnly  bool				 `json:"readOnly"`
	ProgramID ids.ID 			 `json:"programID"`
	Method		string 			 `json:"string"`
	Data		  []byte 			 `json:"data,omitempty"`;
	MaxUnits  uint64 			 `json:"maxUnits"`;
	Require   *Require `json:"require,omitempty" yaml:"require,omitempty"`
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

func (c *CallStep) Execute(ctx context.Context, log logging.Logger, db *state.SimpleMutable, resp *Response) error {
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
		return fmt.Errorf("%w", ErrResultAssertionFailed)
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

var _ Operation = (*CreateStep)(nil)

type CreateStep struct {
	Path string;
}

func (c *CreateStep) Execute(ctx context.Context, log logging.Logger, db *state.SimpleMutable, resp *Response) error {
	// get program path from params
	id, err := programCreateFunc(ctx, db, c.Path)
	if err != nil {
		return err
	}
	resp.SetTxID(id.String())
	resp.SetTimestamp(time.Now().Unix())

	return nil
}

