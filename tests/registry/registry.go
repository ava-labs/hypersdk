// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package registry

import (
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/tests/workload"
)

type TestFunc func(t require.TestingT, tn workload.TestNetwork)

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
	if r == nil {
		return []namedTest{}
	}
	return r.tests
}

// we need to pre-register all the test registries that are created externally in order to comply with the ginko execution order.
// i.e. the global `var _ = ginkgo.Describe` used in the integration/e2e tests need to have this field populated before the iteration
// over the top level nodes.
var testRegistries = map[*Registry]struct{}{}

func Register(registry *Registry, name string, f TestFunc) bool {
	registry.Add(name, f)
	testRegistries[registry] = struct{}{}
	return true
}

func GetTestsRegistries() map[*Registry]struct{} {
	return testRegistries
}
