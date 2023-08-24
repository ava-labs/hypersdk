// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
	"github.com/spf13/cobra"
)

const (
	HRP           = "simulator"
	programPrefix = 0x0
)

var programCmd = &cobra.Command{
	Use: "program",
	RunE: func(*cobra.Command, []string) error {
		return ErrMissingSubcommand
	},
}

var programCreateCmd = &cobra.Command{
	Use:   "create [path to wasm program]",
	Short: "Creates a program from a wasm file and returns the program ID",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return ErrInvalidArgs
		}
		return nil
	},
	RunE: func(_ *cobra.Command, args []string) error {
		filePath := args[0]
		fileBytes, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}

		// spoof txID
		txID := ids.GenerateTestID()

		err = setProgram(db, txID, pubKey, []byte(functions), fileBytes)
		if err != nil {
			return err
		}
		utils.Outf("{{green}}created program tx successful:{{/}} %s\n", txID)
		return nil
	},
}

var programInvokeCmd = &cobra.Command{
	Use:   "invoke [options]",
	Short: "Invokes a wasm program stored on disk",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if programID == "" {
			return fmt.Errorf("program --id cannot be empty")
		}
		return nil
	},

	RunE: func(_ *cobra.Command, args []string) error {
		id, err := ids.FromString(programID)
		if err != nil {
			return err
		}
		exists, owner, functions, program, err := getProgram(db, id)
		if !exists {
			return fmt.Errorf("program %s does not exist", id)
		}
		if err != nil {
			return err
		}

		fmt.Println("owner", owner)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// example cost map
		costMap := map[string]uint64{
			"ConstI32 0x0": 1,
			"ConstI64 0x0": 2,
		}
		var maxGas uint64 = 3000
		storage := newProgramStorage(db)
		runtime := runtime.New(log, runtime.NewMeter(log, maxGas, costMap), storage)
		defer runtime.Stop(ctx)

		err = runtime.Initialize(ctx, program, functions)
		if err != nil {
			return err
		}

		var params []uint64
		// push caller onto stack
		caller, err := runtime.WriteGuestBuffer(ctx, pubKey[:])
		if err != nil {
			return err
		}

		params = append(params, caller)
		resp, err := runtime.Call(ctx, functionName, params...)
		if err != nil {
			return err
		}

		utils.Outf("{{green}}response:{{/}} %v\n", resp)
		return nil
	},
}

func programKey(asset ids.ID) (k []byte) {
	k = make([]byte, 1+consts.IDLen)
	k[0] = programPrefix
	copy(k[1:], asset[:])
	return
}

// [programID] -> [exists, owner, functions, payload]
func getProgram(
	db database.Database,
	programID ids.ID,
) (
	bool, // exists
	ed25519.PublicKey, // owner
	[]string, // functions
	[]byte, // program bytes
	error,
) {
	k := programKey(programID)
	v, err := db.Get(k)
	if errors.Is(err, database.ErrNotFound) {
		return false, ed25519.EmptyPublicKey, nil, nil, nil
	}
	if err != nil {
		return false, ed25519.EmptyPublicKey, nil, nil, err
	}
	var owner ed25519.PublicKey
	copy(owner[:], v[:ed25519.PublicKeyLen])

	functionLen := binary.BigEndian.Uint32(v[ed25519.PublicKeyLen : ed25519.PublicKeyLen+consts.Uint32Len])

	var functionBytes []byte
	copy(functionBytes[:], v[ed25519.PublicKeyLen+consts.Uint32Len:ed25519.PublicKeyLen+consts.Uint32Len+functionLen])

	functions := strings.Split(string(functionBytes), ",")

	var program []byte
	copy(program[:], v[ed25519.PublicKeyLen+consts.Uint32Len+functionLen:])
	return true, owner, functions, program, nil
}

// [owner]
// [functions length]
// [functions]
// [program]
func setProgram(
	db database.Database,
	programID ids.ID,
	owner ed25519.PublicKey,
	functions []byte,
	program []byte,
) error {
	k := programKey(programID)
	functionLen := len(functions)
	v := make([]byte, ed25519.PublicKeyLen+consts.Uint32Len+functionLen+len(program))
	copy(v, owner[:])
	fmt.Printf("owner: %v\n", v)
	binary.BigEndian.PutUint32(v[ed25519.PublicKeyLen:ed25519.PublicKeyLen+consts.Uint32Len], uint32(functionLen))
	fmt.Printf("add ln: %v\n", v)
	copy(v[ed25519.PublicKeyLen+consts.Uint32Len:], functions[:])
	fmt.Printf("add fn: %v\n", v)
	copy(v[ed25519.PublicKeyLen+consts.Uint32Len+functionLen:], program[:])
	fmt.Printf("add program: %v,\n %v\n", k, v)
	return db.Put(k, v)
}

var _ runtime.Storage = (*programStorage)(nil)

// newProgramStorage returns an instance of runtime storage used for examples
// and backed by memDb.
func newProgramStorage(db database.Database) *programStorage {
	return &programStorage{
		db:            db,
		programPrefix: 0x0,
	}
}

type programStorage struct {
	db            database.Database
	programPrefix byte
}

func (p *programStorage) Get(_ context.Context, id uint32) (bool, ed25519.PublicKey, []string, []byte, error) {
	buf := make([]byte, consts.IDLen)
	binary.BigEndian.PutUint32(buf, id)
	exists, owner, functions, payload, err := getProgram(db, ids.ID(buf))
	if !exists {
		return false, ed25519.EmptyPublicKey, nil, nil, fmt.Errorf("program %d does not exist", id)
	}
	if err != nil {
		return false, ed25519.EmptyPublicKey, nil, nil, err
	}

	return exists, owner, functions, payload, err
}

func (p *programStorage) Set(_ context.Context, id uint32, _ uint32, data []byte) error {
	return nil
}

func address(pk ed25519.PublicKey) string {
	return ed25519.Address(HRP, pk)
}

func parseAddress(s string) (ed25519.PublicKey, error) {
	return ed25519.ParseAddress(HRP, s)
}
