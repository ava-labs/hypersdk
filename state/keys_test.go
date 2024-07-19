// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"testing"

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
	tests := []struct {
		name     string
		perm     Permissions
		contains Permissions
		failing  bool
	}{
		{
			name:     "read has read",
			perm:     Read,
			contains: Read,
			failing:  false,
		},
		{
			name:     "read not all",
			perm:     Read,
			contains: All,
			failing:  true,
		},
		{
			name:     "read not write",
			perm:     Read,
			contains: Write,
			failing:  true,
		},
		{
			name:     "rw has read",
			perm:     Read | Write,
			contains: Read,
			failing:  false,
		},
		{
			name:     "allocate has read",
			perm:     Allocate,
			contains: Read,
			failing:  false,
		},
		{
			name:     "allocate not write",
			perm:     Allocate,
			contains: Write,
			failing:  true,
		},
		{
			name:     "write has read",
			perm:     Write,
			contains: Read,
			failing:  false,
		},
		{
			name:     "write not allocate",
			perm:     Write,
			contains: Allocate,
			failing:  true,
		},
		{
			name:     "none has none",
			perm:     None,
			contains: None,
			failing:  false,
		},
		{
			name:     "none not all",
			perm:     None,
			contains: All,
			failing:  true,
		},
		{
			name:     "all has read",
			perm:     All,
			contains: Read,
			failing:  false,
		},
		{
			name:     "all is all",
			perm:     All,
			contains: All,
			failing:  false,
		},
		{
			name:     "all has none",
			perm:     All,
			contains: None,
			failing:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			sample := tt.perm.Has(tt.contains)

			if tt.failing {
				require.False(sample)
			} else {
				require.True(sample)
			}
		})
	}
}
