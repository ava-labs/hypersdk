// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/cli"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/x/programs/examples"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

var keyCmd = &cobra.Command{
	Use:   "key",
	Short: "manage key",
	RunE: func(*cobra.Command, []string) error {
		return ErrMissingSubcommand
	},
}

var genKeyCmd = &cobra.Command{
	Use: "generate",
	RunE: func(*cobra.Command, []string) error {
		priv, err := ed25519.GeneratePrivateKey()
		if err != nil {
			return err
		}
		utils.Outf("{{green}}created new private key with public address:{{/}} %s\n", keyHRP(priv))
		return setKey(db, priv)
	},
}

func keyHRP(privateKey ed25519.PrivateKey) string {
	return HRP + privateKey.ToHex()[0:3]
}

// setKey sets the private key mapped to the given [publicKey]
func setKey(db database.Database, privateKey ed25519.PrivateKey) error {
	publicKey := privateKey.PublicKey()
	k := make([]byte, 1+ed25519.PublicKeyLen)
	k[0] = keyPrefix
	copy(k[1:], publicKey[:])
	has, err := db.Has(k)
	if err != nil {
		return err
	}
	if has {
		return cli.ErrDuplicate
	}
	err = db.Put(k, privateKey[:])
	if err != nil {
		return err
	}
	return db.Put([]byte(keyHRP(privateKey)), privateKey[:])
}

// getKey gets the private key mapped to the given [publicKey]
func getKey(db database.Database, publicKey ed25519.PublicKey) (ed25519.PrivateKey, error) {
	k := make([]byte, 1+ed25519.PublicKeyLen)
	k[0] = keyPrefix
	copy(k[1:], publicKey[:])
	v, err := db.Get(k)
	if errors.Is(err, database.ErrNotFound) {
		return ed25519.EmptyPrivateKey, nil
	}
	if err != nil {
		return ed25519.EmptyPrivateKey, err
	}
	return ed25519.PrivateKey(v), nil
}

// GetKeys loops through the keys in the database and returns the first [x] keys, or makes them if not enough
func GetKeys(db database.Database, numKeys int) ([]string, error) {
	keys := make([]string, numKeys)
	// loop through db
	iter := db.NewIteratorWithPrefix([]byte(HRP))
	defer iter.Release()

	if iter.Error() != nil {
		fmt.Println("Error: ", iter.Error())
		// return nil, iter.Error()
	}

	i := 0
	for iter.Next() {
		if iter.Key() == nil {
			fmt.Println("nil key")
			continue
		}
		keys[i] = string(iter.Key()[:])
		i++
	}

	// fill if needed
	for j := i; j < numKeys; j++ {
		priv, err := ed25519.GeneratePrivateKey()
		if err != nil {
			return nil, err
		}
		err = setKey(db, priv)
		if err != nil {
			return nil, err
		}
		keys[j] = keyHRP(priv)
	}

	return keys, nil
}

type data map[string]int

// Just for the demo! Not fast at all :(
func GetPrograms(db database.Database, log logging.Logger) (map[uint64]data, error) {
	// loop through db
	iter := db.NewIteratorWithPrefix([]byte(runtime.GetProgramPrefix()))
	defer iter.Release()

	if iter.Error() != nil {
		return nil, iter.Error()
	}
	programs := make(map[uint64]data)
	for iter.Next() {
		if iter.Key() == nil {
			continue
		}
		// get the value from the key
		programID := binary.BigEndian.Uint64(iter.Key()[1:])
		programBytes := iter.Value()

		// grab data from program
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		runtime := runtime.New(log, runtime.NewMeter(log, examples.DefaultMaxFee, examples.CostMap), db)
		defer runtime.Stop(ctx)

		err := runtime.Initialize(ctx, programBytes)
		if err != nil {
			return nil, err
		}
		data, err := runtime.GetUserData()
		if err != nil {
			return nil, err
		}
		programs[programID] = data
	}

	return programs, nil
}

// GetPublicKey gets the public key mapped to the given [keyHRP]
func GetPublicKey(db database.Database, keyHRP string) (ed25519.PublicKey, error) {
	v, err := db.Get([]byte(keyHRP))
	if errors.Is(err, database.ErrNotFound) {
		return ed25519.EmptyPublicKey, nil
	}

	if err != nil {
		return ed25519.EmptyPublicKey, err
	}
	return ed25519.PublicKey(v), nil
}
