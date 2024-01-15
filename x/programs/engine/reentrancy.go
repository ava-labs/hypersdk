// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package engine

import "fmt"

// type reentrancyMap map[ids.ID]map[string]uint32

// assuming this is only used on one program
type reentrancyMap map[string]uint32

type ReentrancyGaurd struct {
	m reentrancyMap
	TempNumber int
}


func NewReentrancyGaurd() *ReentrancyGaurd {
	return &ReentrancyGaurd{
		m: make(reentrancyMap),
	}
}

func (r *ReentrancyGaurd) Reset() {
	r.m = make(reentrancyMap)
}

// func (r *ReentrancyGaurd) Enter(id ids.ID, fn string) {
// 	if _, ok := r.m[id]; !ok {
// 		r.m[id] = make(map[string]uint32)
// 	}
// 	r.m[id][fn]++
// }

// Allow allows a function to be reentered once, or if reentrancy is set don't update the map
func (r *ReentrancyGaurd) Allow(fn string) {
	// if already set throw an error
	if _, ok := r.m[fn]; ok {
		return
	}
	r.m[fn] = 1
}

// Set sets the amount of times a function can be reentered
func (r *ReentrancyGaurd) Set(fn string, val uint32) error {
	// if already set throw an error
	if _, ok := r.m[fn]; ok {
		return fmt.Errorf("cannot reset reentrancy for %s", fn)
	}
	r.m[fn] = val
	return nil
}

func (r *ReentrancyGaurd) Enter(fn string) error {
	fmt.Printf("entering %s\n", fn)
	if _, ok := r.m[fn]; !ok {
		return fmt.Errorf("cannot enter %s, not set for reentrancy", fn)
	}

	if r.m[fn] == 0 {
		return fmt.Errorf("cannot enter %s, reentrancy limit reached", fn)
	}

	r.m[fn]--
	
	return nil
}
