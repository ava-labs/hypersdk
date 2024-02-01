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
		canRead    bool
		canAlloc   bool
		canWrite   bool
	}{
		{
			name:       "key is given Read, Write",
			key:        "test",
			permission: Permission(Read, Write),
			canRead:    true,
			canAlloc:   false,
			canWrite:   true,
		},
		{
			name:       "key is just given allocate",
			key:        "test1",
			permission: Permission(Allocate),
			canRead:    false,
			canAlloc:   true,
			canWrite:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			keys := Keys{tt.key: tt.permission}

			// Check permission
			perm := keys[tt.key]
			require.Equal(tt.canRead, perm.HasPermission(Read))
			require.Equal(tt.canAlloc, perm.HasPermission(Allocate))
			require.Equal(tt.canWrite, perm.HasPermission(Write))
		})
	}
}

func TestUnionPermissions(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		permission1 Permissions
		permission2 Permissions
		canRead     bool
		canAlloc    bool
		canWrite    bool
	}{
		{
			name:        "key has no perms then Read, Write",
			key:         "test",
			permission1: Permission(),
			permission2: Permission(Read, Write),
			canRead:     true,
			canAlloc:    false,
			canWrite:    true,
		},
		{
			name:        "key has Read then Allocate and Write",
			key:         "test1",
			permission1: Permission(Read),
			permission2: Permission(Allocate, Write),
			canRead:     true,
			canAlloc:    true,
			canWrite:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			keys := Keys{tt.key: tt.permission1}

			// Add new permission this should test the Union part
			keys.Add(tt.key, tt.permission2)

			// Check updated positions
			perm := keys[tt.key]
			require.Equal(tt.canRead, perm.HasPermission(Read))
			require.Equal(tt.canAlloc, perm.HasPermission(Allocate))
			require.Equal(tt.canWrite, perm.HasPermission(Write))
		})
	}
}
