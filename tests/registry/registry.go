// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package registry

import (
	"github.com/ava-labs/hypersdk/tests/workload"
	"github.com/onsi/ginkgo/v2"
)

type TestFunc func(t ginkgo.FullGinkgoTInterface, tn workload.TestNetwork) error

type namedTest struct {
	Fnc  TestFunc
	Name string
}
type Registry struct {
	tests []namedTest
}

func (r *Registry) Add(name string, f TestFunc) {
	r.tests = append(r.tests, namedTest{Fnc: f, Name: name})
}

func (r *Registry) List() []namedTest {
	return r.tests
}
