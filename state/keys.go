// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/keys"
)

var errInvalidHexadecimalString = errors.New("invalid hexadecimal string")

var (
	_                   Scope = (*Keys)(nil)
	_                   Scope = (*SimulatedKeys)(nil)
	CompletePermissions Scope = fullAccess{}
)

const (
	Read     Permissions = 1
	Allocate             = 1<<1 | Read
	Write                = 1<<2 | Read

	None Permissions = 0
	All              = Read | Allocate | Write
)

type Scope interface {
	Has(key []byte, perm Permissions) bool
}

type fullAccess struct{}

func (fullAccess) Has([]byte, Permissions) bool { return true }

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

func (k Keys) Has(key []byte, permission Permissions) bool {
	return k[string(key)].Has(permission)
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

// WithoutPermissions returns the keys of k as a slice with permissions removed
func (k Keys) WithoutPermissions() []string {
	ks := make([]string, len(k))
	for key := range k {
		ks = append(ks, key)
	}
	return ks
}

type keysJSON map[string]Permissions

// MarshalJSON marshals Keys as readable JSON.
// Keys are hex encoded strings and permissions
// are either valid named strings or unknown hex encoded strings.
func (k Keys) MarshalJSON() ([]byte, error) {
	kJSON := make(keysJSON)
	for key, perm := range k {
		hexKey, err := codec.Bytes(key).MarshalText()
		if err != nil {
			return nil, err
		}
		kJSON[string(hexKey)] = perm
	}
	return json.Marshal(kJSON)
}

// UnmarshalJSON unmarshals readable JSON.
func (k *Keys) UnmarshalJSON(b []byte) error {
	// Initialize map if nil
	if *k == nil {
		*k = make(Keys)
	}
	var keysJSON keysJSON
	if err := json.Unmarshal(b, &keysJSON); err != nil {
		return err
	}
	for hexKey, perm := range keysJSON {
		var key codec.Bytes
		err := key.UnmarshalText([]byte(hexKey))
		if err != nil {
			return err
		}
		(*k)[string(key)] = perm
	}
	return nil
}

type SimulatedKeys Keys

func (d SimulatedKeys) Has(key []byte, perm Permissions) bool {
	Keys(d).Add(string(key), perm)
	return true
}

func (d SimulatedKeys) StateKeys() Keys {
	return Keys(d)
}

func (p *Permissions) UnmarshalText(in []byte) error {
	switch str := strings.ToLower(string(in)); str {
	case "read":
		*p = Read
	case "write":
		*p = Write
	case "allocate":
		*p = Allocate
	case "all":
		*p = All
	case "none":
		*p = None
	default:
		res, err := hex.DecodeString(str)
		if err != nil || len(res) != 1 {
			return fmt.Errorf("permission %s: %w", str, errInvalidHexadecimalString)
		}
		*p = Permissions(res[0])
	}
	return nil
}

func (p Permissions) MarshalText() ([]byte, error) {
	return []byte(p.String()), nil
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
