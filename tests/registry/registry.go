// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package registry

import (
	"github.com/onsi/ginkgo/v2"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/tests/workload"
)

type TestFunc func(t ginkgo.FullGinkgoTInterface, tn workload.TestNetwork, authFactories ...chain.AuthFactory)

type namedTest struct {
	Fnc           TestFunc
	Name          string
	AuthFactories []chain.AuthFactory
}
type Registry struct {
	tests                []namedTest
	requestedAllocations []*genesis.CustomAllocation
}

func (r *Registry) Add(name string, f TestFunc, requestedBalances ...uint64) {
	allocations := make([]*genesis.CustomAllocation, 0, len(requestedBalances))
	authFactories := make([]chain.AuthFactory, 0, len(requestedBalances))
	for _, requestedBalance := range requestedBalances {
		private, _ := ed25519.GeneratePrivateKey()
		authFactory := auth.NewED25519Factory(private)
		authFactories = append(authFactories, authFactory)
		allocations = append(allocations, &genesis.CustomAllocation{Address: authFactory.Address(), Balance: requestedBalance})
	}
	r.tests = append(r.tests, namedTest{Fnc: f, Name: name, AuthFactories: authFactories})
	r.requestedAllocations = append(r.requestedAllocations, allocations...)
}

func (r *Registry) List() []namedTest {
	if r == nil {
		return []namedTest{}
	}
	return r.tests
}

func (r *Registry) GetRequestedAllocations() []*genesis.CustomAllocation {
	return r.requestedAllocations
}

// we need to pre-register all the test registries that are created externally in order to comply with the ginko execution order.
// i.e. the global `var _ = ginkgo.Describe` used in the integration/e2e tests need to have this field populated before the iteration
// over the top level nodes.
var testRegistries = map[*Registry]bool{}

func Register(registry *Registry, name string, f TestFunc, requestedBalances ...uint64) bool {
	registry.Add(name, f, requestedBalances...)
	testRegistries[registry] = true
	return true
}

func GetTestsRegistries() map[*Registry]bool {
	return testRegistries
}
