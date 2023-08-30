// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
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
	HRP       = "simulator"
	HRP_KEY   = "sim_key_"
	keyPrefix = 0x1
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
		programId, err := initalizeProgram(fileBytes, pk)
		if err != nil {
			return err
		}

		err = runtime.SetProgram(db, programId, pk, fileBytes)
		if err != nil {
			return err
		}
		utils.Outf("{{green}}create program action successful program id:{{/}} %v\n", programId)
		return nil
	},
}

func initalizeProgram(programBytes []byte, caller ed25519.PublicKey) (uint64, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runtime := runtime.New(log, runtime.NewMeter(log, maxFee, costMap), db, caller)
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
		exists, owner, program, err := runtime.GetProgram(db, programID)
		if !exists {
			return fmt.Errorf("program %v does not exist", programID)
		}
		if err != nil {
			return err
		}

		fmt.Println("owner", owner)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// TODO: owner for now, change to caller later
		runtime := runtime.New(log, runtime.NewMeter(log, maxFee, costMap), db, owner)
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
				case strings.HasPrefix(p, HRP_KEY):
					// address
					pk, err := getPublicKey(db, p)
					fmt.Println("pk", pk)
					if err != nil {
						return err
					}
					ptr, err := runtime.WriteGuestBuffer(ctx, pk[:])
					if err != nil {
						return err
					}
					callParams = append(callParams, ptr)
				default:
					// treat like a number
					var num uint64
					num, err := strconv.ParseUint(p, 10, 64)

					if err != nil {
						return err
					}
					callParams = append(callParams, num)
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
	exists, owner, payload, err := runtime.GetProgram(db, uint64(id))
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
