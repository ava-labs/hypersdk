// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package engine

import (
	"errors"

	"github.com/ava-labs/avalanchego/ids"
)

var ErrReentrancy = errors.New("reentrant call not allowed")

type reentrancyMap map[ids.ID]map[string]bool

// ReentrancyGuard keeps track of called methods during execution.
type ReentrancyGuard struct {
	// In the future, every call context might need its own reentrancyMap
	// with fine grain locking across call contexts if we want to parellellize program calls.
	m reentrancyMap
}

func NewReentrancyGuard() *ReentrancyGuard {
	return &ReentrancyGuard{
		m: make(reentrancyMap),
	}
}

// Reset resets the reentrancy map.
func (r *ReentrancyGuard) Reset() {
	r.m = make(reentrancyMap)
}

// Note, you can bypass this check by calling a different method for the same program which call the intended no reenter method.
// ex. Program A has method no_reenter and reenter. Reenter calls method no_reenter.
// If an external call wants to call no_reenter, it can call reenter instead and will bypass the reentrancy check.
// The second call wont be an external program call, but it will be a reentrant call and is not handled.

// Enter returns if we have never entered this function before, errors otherwise.
// If the function has not been entered before, we set the visited flag.
// We can return an error because only non-reentrant functions will call this method.
func (r *ReentrancyGuard) Enter(id ids.ID, fn string) error {
	// reentering since we already visited
	if r.m[id] != nil && r.m[id][fn] {
		return ErrReentrancy
	}

	// set visited
	if _, ok := r.m[id]; !ok {
		r.m[id] = make(map[string]bool)
	}
	r.m[id][fn] = true

	return nil
}
