// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ava-labs/hypersdk/keys"
)

var errInvalidHexadecimalString = errors.New("invalid hexadecimal string")

const (
	Read     Permissions = 1
	Allocate             = 1<<1 | Read
	Write                = 1<<2 | Read

	None Permissions = 0
	All              = Read | Allocate | Write
)

// StateKey holds the name of the key and its permission (Read/Allocate/Write). By default,
// initialization of Keys with duplicate key will not work. And to prevent duplicate
// insertions from overriding the original permissions, use the Add function below.
type Keys map[string]Permissions

// All acceptable permission options
type Permissions byte

// Add verifies that a key is well-formatted and adds it to the conflict set.
//
// If the key already exists, the permissions are unioned.
func (k Keys) Add(key string, permission Permissions) bool {
	// If a key is not properly formatted, it cannot be added.
	if !keys.Valid(key) {
		return false
	}

	// Transaction's permissions are the union of all
	// state keys from both Actions and Auth
	k[key] |= permission
	return true
}

// Returns the chunk sizes of each key
func (k Keys) ChunkSizes() ([]uint16, bool) {
	chunks := make([]uint16, 0, len(k))
	for key := range k {
		chunk, ok := keys.DecodeChunks([]byte(key))
		if !ok {
			return nil, false
		}
		chunks = append(chunks, chunk)
	}
	return chunks, true
}

type permsJSON []string

type keysJSON struct {
	Perms map[string]permsJSON
}

func (k Keys) MarshalJSON() ([]byte, error) {
	keysJSON := keysJSON{
		Perms: make(map[string]permsJSON),
	}
	for key, perm := range k {
		keysJSON.Perms[perm.String()] = append(keysJSON.Perms[perm.String()], hex.EncodeToString([]byte(key)))
	}
	return json.Marshal(keysJSON)
}

func (k *Keys) UnmarshalJSON(b []byte) error {
	var keysJSON keysJSON
	if err := json.Unmarshal(b, &keysJSON); err != nil {
		return err
	}
	for strPerm, keyList := range keysJSON.Perms {
		perm, err := newPermissionFromString(strPerm)
		if err != nil {
			return err
		}
		if perm < None || perm > All {
			return fmt.Errorf("invalid permission encoded in json %d", perm)
		}
		for _, encodedKey := range keyList {
			key, err := hex.DecodeString(encodedKey)
			if err != nil {
				return err
			}
			(*k)[string(key)] = perm
		}
	}
	return nil
}

func newPermissionFromString(s string) (Permissions, error) {
	var perm Permissions
	switch s {
	case "read":
		perm = Read
	case "write":
		perm = Write
	case "allocate":
		perm = Allocate
	case "all":
		perm = All
	case "none":
		perm = None
	default:
		res, err := hex.DecodeString(s)
		if err != nil || len(res) != 1 {
			return 0, fmt.Errorf("permission %s: %w", s, errInvalidHexadecimalString)
		}
		perm = Permissions(res[0])
	}
	return perm, nil
}

// Has returns true if [p] has all the permissions that are contained in require
func (p Permissions) Has(require Permissions) bool {
	return require&^p == 0
}

func (p Permissions) String() string {
	switch p {
	case Read:
		return "read"
	case Write:
		return "write"
	case Allocate:
		return "allocate"
	case All:
		return "all"
	case None:
		return "none"
	default:
		return hex.EncodeToString([]byte{byte(p)})
	}
}
