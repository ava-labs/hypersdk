// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"os"
	"strconv"
	"strings"

	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/x/programs/examples"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
	"github.com/spf13/cobra"
)

const (
	HRP = "sim_key_"
	// keyPrefix that stores pub key -> private key mapping
	keyPrefix = 0x1
)

var programCmd = &cobra.Command{
	Use: "program",
	RunE: func(*cobra.Command, []string) error {
		return ErrMissingSubcommand
	},
}

var programCreateCmd = &cobra.Command{
	Use:   "create [path to wasm program]",
	Short: "Creates a program from a wasm file, calls init(if exists) and returns the program ID",
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

		programId, err := InitializeProgram(fileBytes)
		if err != nil {
			return err
		}

		err = runtime.SetProgram(db, programId, fileBytes)
		if err != nil {
			return err
		}
		utils.Outf("{{green}}create program action successful program id:{{/}} %v\n", programId)
		return nil
	},
}

func InitializeProgram(programBytes []byte) (uint64, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runtime := runtime.New(log, runtime.NewMeter(log, maxFee, examples.CostMap), db)
	defer runtime.Stop(ctx)

	programID, err := runtime.Create(ctx, programBytes)
	if err != nil {
		return 0, err
	}
	return programID, nil
}

var programInvokeCmd = &cobra.Command{
	Use:   "invoke [options]",
	Short: "Invokes a deployed wasm program",
	PreRunE: func(cmd *cobra.Command, args []string) (err error) {
		if programID == 0 {
			return ErrMissingProgramID
		}
		if callerAddress == "" {
			return ErrMissingAddress
		}
		if functionName == "" {
			return ErrMissingFunctionName
		}
		return nil
	},

	RunE: func(_ *cobra.Command, args []string) error {
		exists, program, err := runtime.GetProgram(db, programID)
		if !exists {
			return ErrInvalidProgramID
		}
		if err != nil {
			return err
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		runtime := runtime.New(log, runtime.NewMeter(log, maxFee, examples.CostMap), db)
		defer runtime.Stop(ctx)

		err = runtime.Initialize(ctx, program)
		if err != nil {
			return err
		}

		var callParams []uint64
		if params != "" {
			for _, param := range strings.Split(params, ",") {
				switch p := strings.ToLower(param); {
				case p == "true":
					callParams = append(callParams, 1)
				case p == "false":
					callParams = append(callParams, 0)
				case strings.HasPrefix(p, HRP):
					// public key
					pk, err := GetPublicKey(db, p)
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
