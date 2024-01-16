// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package engine

import "fmt"

// TODO: change to this map
// type reentrancyMap map[ids.ID]map[string]uint32

// assuming this is only used on one program
// limit reentrancy to 255 times
type reentrancyMap map[string]uint8

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
func (r *ReentrancyGaurd) Allow(fn string) {
	// if already set throw an error
	if _, ok := r.m[fn]; ok {
		fmt.Println("reentrancy already set for function, ", r.m[fn])
		return
	}
	r.m[fn] = 1
}

// Set sets the amount of times a function can be reentered
func (r *ReentrancyGaurd) Set(fn string, val uint8) {
	// if already set throw an error
	if _, ok := r.m[fn]; ok {
		fmt.Println("trying to set reentrancy for a function that already has reentrancy set. value is ", r.m[fn])
		// should log a warning here tho... 
		// or potentially set the value if it's less than the current value
		return
	}
	r.m[fn] = val
}

func (r *ReentrancyGaurd) Enter(fn string) error {
	if _, ok := r.m[fn]; !ok {
		return fmt.Errorf("cannot enter %s, not set for reentrancy", fn)
	}

	if r.m[fn] == 0 {
		return fmt.Errorf("cannot enter %s, reentrancy limit reached", fn)
	}

	r.m[fn]--
	fmt.Println("decreased to ", r.m[fn])
	return nil
}
