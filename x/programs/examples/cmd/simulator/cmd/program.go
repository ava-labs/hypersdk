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
	"time"

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
	HRP_KEY       = "simulator_key_"
	programPrefix = 0x0
	keyPrefix     = 0x1
)

// example cost map
var costMap = map[string]uint64{
	"ConstI32 0x0": 1,
	"ConstI64 0x0": 2,
}

var programCmd = &cobra.Command{
	Use: "program",
	RunE: func(*cobra.Command, []string) error {
		return ErrMissingSubcommand
	},
}

var programCreateCmd = &cobra.Command{
	Use:   "create [path to wasm program]",
	Short: "Creates a program from a wasm file, calls init_program and returns the program ID",
	PreRunE: func(cmd *cobra.Command, args []string) (err error) {
		if len(args) != 1 {
			return ErrInvalidArgs
		}
		if callerAddress == "" {
			return ErrMissingAddress
		}
		return nil
	},
	RunE: func(_ *cobra.Command, args []string) error {
		filePath := args[0]
		fileBytes, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}

		pk, err := getPublicKey(db, callerAddress)
		if err != nil {
			return err
		}

		// getKey(db, pk)
		programId, err := initalizeProgram(fileBytes)
		if err != nil {
			return err
		}

		err = setProgram(db, programId, pk, fileBytes)
		if err != nil {
			return err
		}
		utils.Outf("{{green}}create program action successful program id:{{/}} %v\n", programId)
		return nil
	},
}

func initalizeProgram(programBytes []byte) (uint64, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runtime := runtime.New(log, runtime.NewMeter(log, maxFee, costMap), &db)
	defer runtime.Stop(ctx)

	programID, err := runtime.Create(ctx, programBytes)
	if err != nil {
		return 0, err
	}
	fmt.Println("programID", programID)
	return programID, nil
}

var programInvokeCmd = &cobra.Command{
	Use:   "invoke [options]",
	Short: "Invokes a wasm program stored on disk",
	PreRunE: func(cmd *cobra.Command, args []string) (err error) {
		if programID == 0 {
			return fmt.Errorf("program --id cannot be empty")
		}
		if callerAddress == "" {
			return ErrMissingAddress
		}
		if functionName == "" {
			return fmt.Errorf("function --name cannot be empty")
		}
		// pubKey, err = parseAddress(callerAddress)
		// if err != nil {
		// 	return err
		// }
		return nil
	},

	RunE: func(_ *cobra.Command, args []string) error {
		// id, err := ids.FromString(programID)
		// if err != nil {
		// 	return err
		// }
		exists, owner, program, err := getProgram(db, programID)
		if !exists {
			return fmt.Errorf("program %v does not exist", programID)
		}
		if err != nil {
			return err
		}

		fmt.Println("owner", owner)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		runtime := runtime.New(log, runtime.NewMeter(log, maxFee, costMap), &db)
		defer runtime.Stop(ctx)

		err = runtime.Initialize(ctx, program)
		if err != nil {
			return err
		}

		fmt.Println("params", params)
		var callParams []uint64
		if params != "" {
			for _, param := range strings.Split(params, ",") {
				switch p := strings.ToLower(param); {
				case p == "true":
					callParams = append(callParams, 1)
				case p == "false":
					callParams = append(callParams, 0)
				case strings.HasPrefix(p, HRP):
					// address
					pk, err := parseAddress(p)
					if err != nil {
						return err
					}
					ptr, err := runtime.WriteGuestBuffer(ctx, pk[:])
					if err != nil {
						return err
					}
					callParams = append(callParams, ptr)
				default:
					id, _ := ids.FromString(p)
					// id
					if id != ids.Empty {
						ptr, err := runtime.WriteGuestBuffer(ctx, id[:])
						if err != nil {
							return err
						}
						callParams = append(callParams, ptr)
					} else {
						return fmt.Errorf("param not handled please implement: %s", param)
					}
				}
			}
		}
		// prepend programID
		callParams = append([]uint64{programID}, callParams...)

		resp, err := runtime.Call(ctx, functionName, callParams...)
		if err != nil {
			return err
		}

		utils.Outf("{{green}}response:{{/}} %v\n", resp)
		return nil
	},
}

func programKey(asset uint64) (k []byte) {
	k = make([]byte, 1+consts.IDLen)
	// convert uint64 to bytes
	binary.BigEndian.PutUint64(k[1:], asset)
	k[0] = programPrefix
	return
}

// [programID] -> [exists, owner, functions, payload]
func getProgram(
	db database.Database,
	programID uint64,
) (
	bool, // exists
	ed25519.PublicKey, // owner
	[]byte, // program bytes
	error,
) {
	k := programKey(programID)
	v, err := db.Get(k)
	if errors.Is(err, database.ErrNotFound) {
		return false, ed25519.EmptyPublicKey, nil, nil
	}
	if err != nil {
		return false, ed25519.EmptyPublicKey, nil, err
	}
	var owner ed25519.PublicKey
	copy(owner[:], v[:ed25519.PublicKeyLen])
	programLen := uint32(len(v)) - ed25519.PublicKeyLen
	program := make([]byte, programLen)
	copy(program[:], v[ed25519.PublicKeyLen:])
	return true, owner, program, nil
}

// [owner]
// [program]
func setProgram(
	db database.Database,
	programID uint64,
	owner ed25519.PublicKey,
	program []byte,
) error {
	k := programKey(programID)
	v := make([]byte, ed25519.PublicKeyLen+len(program))
	fmt.Println("Owner set: ", owner)
	copy(v, owner[:])
	copy(v[ed25519.PublicKeyLen:], program[:])
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

func (p *programStorage) Get(_ context.Context, id uint32) (bool, ed25519.PublicKey, []byte, error) {
	buf := make([]byte, consts.IDLen)
	binary.BigEndian.PutUint32(buf, id)
	exists, owner, payload, err := getProgram(db, uint64(id))
	if !exists {
		return false, ed25519.EmptyPublicKey, nil, fmt.Errorf("program %d does not exist", id)
	}
	if err != nil {
		return false, ed25519.EmptyPublicKey, nil, err
	}

	return exists, owner, payload, err
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

func fakeID() ids.ID {
	return ids.Empty.Prefix(uint64(time.Now().UnixNano()))
}
