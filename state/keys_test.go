// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"testing"

	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/stretchr/testify/require"
)

func TestAddPermissions(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		permission Permissions
	}{
		{
			name:       "key is given Read, Write",
			key:        "test",
			permission: Read | Write,
		},
		{
			name:       "key is just given allocate",
			key:        "test1",
			permission: Allocate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			keys := make(Keys)
			require.True(keys.Add(tt.key, tt.permission))

			// Check permission
			perm := keys[tt.key]
			require.Equal(tt.permission, perm)
		})
	}
}

func TestUnionPermissions(t *testing.T) {
	tests := []struct {
		name               string
		key                string
		permission1        Permissions
		permission2        Permissions
		expectedPermission Permissions
	}{
		{
			name:               "key has no perms then Read, Write",
			key:                "test",
			permission1:        None,
			permission2:        Read | Write,
			expectedPermission: Read | Write,
		},
		{
			name:               "key has Read then Allocate and Write",
			key:                "test1",
			permission1:        Read,
			permission2:        Allocate | Write,
			expectedPermission: Read | Allocate | Write,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			keys := Keys{tt.key: tt.permission1}

			// Add new permission this should test the Union part
			require.True(keys.Add(tt.key, tt.permission2))

			// Check updated positions
			perm := keys[tt.key]
			require.Equal(tt.expectedPermission, perm)
		})
	}
}

func TestMalformedKey(t *testing.T) {
	require := require.New(t)
	keys := make(Keys)
	require.False(keys.Add("", Read))
}

func TestHasPermissions(t *testing.T) {
	allPerms := set.NewSet[Permissions](5)
	allPerms.Add(Read, Allocate, Write, None, All)

	tests := []struct {
		name string
		perm Permissions
		has  set.Set[Permissions]
	}{
		{
			name: "read",
			perm: Read,
			has: func() set.Set[Permissions] {
				permSet := set.NewSet[Permissions](2)
				permSet.Add(Read, None)
				return permSet
			}(),
		},
		{
			name: "allocate",
			perm: Allocate,
			has: func() set.Set[Permissions] {
				permSet := set.NewSet[Permissions](3)
				permSet.Add(Read, Allocate, None)
				return permSet
			}(),
		},
		{
			name: "write",
			perm: Write,
			has: func() set.Set[Permissions] {
				permSet := set.NewSet[Permissions](3)
				permSet.Add(Read, Write, None)
				return permSet
			}(),
		},
		{
			name: "none",
			perm: None,
			has: func() set.Set[Permissions] {
				permSet := set.NewSet[Permissions](1)
				permSet.Add(None)
				return permSet
			}(),
		},
		{
			name: "all",
			perm: All,
			has: func() set.Set[Permissions] {
				permSet := set.NewSet[Permissions](5)
				permSet.Add(Read, Allocate, Write, None, All)
				return permSet
			}(),
		},
	}

	for _, tt := range tests {
		for perm := range allPerms {
			t.Run(tt.name, func(t *testing.T) {
				require := require.New(t)
				contains := tt.has.Contains(perm)
				has := tt.perm.Has(perm)
				require.Equal(contains, has)
			})
		}
	}
}
