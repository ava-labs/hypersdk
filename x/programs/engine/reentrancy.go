// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package engine

import (
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
)


type reentrancyMap map[ids.ID]map[string]bool

type ReentrancyGaurd struct {
	// potentially needs to have one for every call context 
	// and fine grain lock on call contexts if we want to support calls in parallelb
	m reentrancyMap
}


func NewReentrancyGaurd() *ReentrancyGaurd {
	return &ReentrancyGaurd{
		m: make(reentrancyMap),
	}
}

// Reset resets the reentrancy map. 
func (r *ReentrancyGaurd) Reset() {
	r.m = make(reentrancyMap)
}

// Set sets the amount of times a function can be reentered
func (r *ReentrancyGaurd) Set(id ids.ID, fn string, is_reentrant bool) {
	if _, ok := r.m[id]; !ok {
		r.m[id] = make(map[string]bool)
		r.m[id][fn] = is_reentrant
		return
	}

	// if already set return, and log as warning. 
	if _, ok := r.m[id][fn]; ok {
		fmt.Println("trying to set reentrancy for a function that already has reentrancy set. value is ", r.m[id][fn])
		// should log a warning here tho... 
		// or potentially set the value if it's less than the current value
		return
	}

	r.m[id][fn] = is_reentrant
}


// only issue is if they enter another method that is reentrant, then they can enter this method again from within the other method.
// The second call wont be an external program call, but it will be a reentrant call and is not handled.

// Enter program should only return 1 if we have never entered this function before.
func (r *ReentrancyGaurd) Enter(id ids.ID, fn string) int64 {
	fmt.Println("entering reentrancy gaurd for ",  fn)
	// reentering since we already visited
	if r.m[id] != nil && r.m[id][fn] {
		return 0
	}

	// set visited
	if _, ok := r.m[id]; !ok {
		r.m[id] = make(map[string]bool)
	}
	r.m[id][fn] = true

	return 1
}
