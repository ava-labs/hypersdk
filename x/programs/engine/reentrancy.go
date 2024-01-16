// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package engine

import (
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
)

// TODO: change to this map
type reentrancyMap map[ids.ID]map[string]uint8

// assuming this is only used on one program
// limit reentrancy to 255 times
// type reentrancyMap map[string]uint8

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

// Allow allows a function to be reentered once, or if reentrancy is set don't update the map
func (r *ReentrancyGaurd) Allow(id ids.ID, fn string) {
	r.Set(id, fn, 1)
}

// Set sets the amount of times a function can be reentered
func (r *ReentrancyGaurd) Set(id ids.ID, fn string, val uint8) {
	if _, ok := r.m[id]; !ok {
		r.m[id] = make(map[string]uint8)
		r.m[id][fn] = val
		return
	}

	// if already set return, and log as warning. 
	if _, ok := r.m[id][fn]; ok {
		fmt.Println("trying to set reentrancy for a function that already has reentrancy set. value is ", r.m[id][fn])
		// should log a warning here tho... 
		// or potentially set the value if it's less than the current value
		return
	}

	r.m[id][fn] = val
}

func (r *ReentrancyGaurd) Enter(id ids.ID, fn string) error {
	if _, ok := r.m[id]; !ok {
		return fmt.Errorf("cannot enter %s, not set for reentrancy", fn)
	}

	if _, ok := r.m[id][fn]; !ok {
		return fmt.Errorf("cannot enter %s, not set for reentrancy", fn)
	}

	if r.m[id][fn] == 0 {
		return fmt.Errorf("cannot enter %s, reentrancy limit reached", fn)
	}

	r.m[id][fn]--
	fmt.Println("decreased to ", r.m[id][fn])
	return nil
}
