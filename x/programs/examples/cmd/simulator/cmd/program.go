// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	// "errors"
	"errors"
	"fmt"
	"os"

	// "github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/spf13/cobra"
)

const programPrefix = 0x0

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

		k := prefixProgramKey(txID)
		err = db.Put(k, fileBytes)
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
		if programName == "" {
			return fmt.Errorf("program --name cannot be empty")
		}
		return nil
	},

	RunE: func(_ *cobra.Command, args []string) error {
		utils.Outf("{{green}}created a program:{{/}} %s\n", programName)
		return db.Put([]byte("program"), []byte(args[0]))
	},
}

// func getProgramBytes(
// 	db database.Database,
// 	id uint32,
// ) ([]byte, bool, error) {
// 	k := prefixProgramKey(id)
// 	v, err := db.Get(k)
// 	if errors.Is(err, database.ErrNotFound) {
// 		return nil, false, nil
// 	}
// 	if err != nil {
// 		return nil, false, err
// 	}
// 	return v, true, nil
// }

func programKey(asset ids.ID) (k []byte) {
	k = make([]byte, 1+consts.IDLen)
	k[0] = programPrefix
	copy(k[1:], asset[:])
	return
}

func GetProgram(
	db database.Database,
	program ids.ID,
) (
	bool, // exists
	ids.ID, // in
	uint64, // inTick
	ids.ID, // out
	uint64, // outTick
	uint64, // remaining
	ed25519.PublicKey, // owner
	error,
) {
	k := programKey(program)
	v, err := db.Get(k)
	if errors.Is(err, database.ErrNotFound) {
		return false, ids.Empty, 0, ids.Empty, 0, 0, ed25519.EmptyPublicKey, nil
	}
	if err != nil {
		return false, ids.Empty, 0, ids.Empty, 0, 0, ed25519.EmptyPublicKey, err
	}
	var in ids.ID
	copy(in[:], v[:consts.IDLen])
	inTick := binary.BigEndian.Uint64(v[consts.IDLen:])
	var out ids.ID
	copy(out[:], v[consts.IDLen+consts.Uint64Len:consts.IDLen*2+consts.Uint64Len])
	outTick := binary.BigEndian.Uint64(v[consts.IDLen*2+consts.Uint64Len:])
	supply := binary.BigEndian.Uint64(v[consts.IDLen*2+consts.Uint64Len*2:])
	var owner ed25519.PublicKey
	copy(owner[:], v[consts.IDLen*2+consts.Uint64Len*3:])
	return true, in, inTick, out, outTick, supply, owner, nil
}
