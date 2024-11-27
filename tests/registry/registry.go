// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package registry

import (
	"github.com/onsi/ginkgo/v2"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/tests/workload"
)

type TestFunc func(t ginkgo.FullGinkgoTInterface, tn workload.TestNetwork, authFactories []chain.AuthFactory)

type namedTest struct {
	Fnc               TestFunc
	Name              string
	AuthFactories     []chain.AuthFactory
	requestedBalances []uint64
}
type Registry struct {
	tests []*namedTest
}

func (r *Registry) Add(name string, f TestFunc, requestedBalances ...uint64) {
	r.tests = append(r.tests, &namedTest{Fnc: f, Name: name, requestedBalances: requestedBalances})
}

func (r *Registry) List() []*namedTest {
	if r == nil {
		return []*namedTest{}
	}
	return r.tests
}

func (r *Registry) GenerateCustomAllocations(generateAuthFactory func() (chain.AuthFactory, error)) ([]*genesis.CustomAllocation, error) {
	requestedAllocations := make([]*genesis.CustomAllocation, 0)
	for index, test := range r.tests {
		for _, requestedBalance := range test.requestedBalances {
			authFactory, err := generateAuthFactory()
			if err != nil {
				return nil, err
			}
			test.AuthFactories = append(test.AuthFactories, authFactory)
			requestedAllocations = append(
				requestedAllocations,
				&genesis.CustomAllocation{
					Address: authFactory.Address(),
					Balance: requestedBalance,
				},
			)
		}
		r.tests[index] = test
	}
	return requestedAllocations, nil
}

// we need to pre-register all the test registries that are created externally in order to comply with the ginko execution order.
// i.e. the global `var _ = ginkgo.Describe` used in the integration/e2e tests need to have this field populated before the iteration
// over the top level nodes.
var testRegistries = map[*Registry]bool{}

func Register(registry *Registry, name string, f TestFunc, requestedBalances ...uint64) bool {
	registry.Add(name, withRequiredPrefundedAuthFactories(f, len(requestedBalances)), requestedBalances...)
	testRegistries[registry] = true
	return true
}

func GetTestsRegistries() map[*Registry]bool {
	return testRegistries
}

// withRequiredPrefundedAuthFactories wraps the TestFunc in a new TestFunc adding length validation over the provided authFactories
func withRequiredPrefundedAuthFactories(f TestFunc, requiredLength int) TestFunc {
	return func(t ginkgo.FullGinkgoTInterface, tn workload.TestNetwork, authFactories []chain.AuthFactory) {
		require.Len(t, authFactories, requiredLength, "required pre-funded authFactories have not been initialized")
		f(t, tn, authFactories)
	}
}
