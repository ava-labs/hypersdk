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
		canRead    bool
		canAlloc   bool
		canWrite   bool
	}{
		{
			name:       "key is given Read, Write",
			key:        "test",
			permission: Permissions{Read: true, Write: true},
			canRead:    true,
			canAlloc:   false,
			canWrite:   true,
		},
		{
			name:       "key is just given allocate",
			key:        "test1",
			permission: Permissions{Allocate: true},
			canRead:    false,
			canAlloc:   true,
			canWrite:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			keys := Keys{tt.key: &tt.permission}

			// Check permission
			perm := keys[tt.key]
			require.Equal(tt.canRead, perm.Read)
			require.Equal(tt.canAlloc, perm.Allocate)
			require.Equal(tt.canWrite, perm.Write)
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
			permission1: Permissions{},
			permission2: Permissions{Read: true, Write: true},
			canRead:     true,
			canAlloc:    false,
			canWrite:    true,
		},
		{
			name:        "key has Read then Allocate and Write",
			key:         "test1",
			permission1: Permissions{Read: true},
			permission2: Permissions{Allocate: true, Write: true},
			canRead:     true,
			canAlloc:    true,
			canWrite:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			keys := Keys{tt.key: &tt.permission1}

			// Add new permission this should test the Union part
			keys.Add(tt.key, &tt.permission2)

			// Check updated positions
			perm := keys[tt.key]
			require.Equal(tt.canRead, perm.Read)
			require.Equal(tt.canAlloc, perm.Allocate)
			require.Equal(tt.canWrite, perm.Write)
		})
	}
}

func TestToBytes(t *testing.T) {
	tests := []struct {
		name          string
		key           string
		permission    Permissions
		expectedBytes []byte
	}{
		{
			name:          "key has Read, Allocate, Write",
			key:           "test",
			permission:    Permissions{Read: true, Allocate: true, Write: true},
			expectedBytes: set.NewBits(int(Read), int(Allocate), int(Write)).Bytes(),
		},
		{
			name:          "key has no perms",
			key:           "test1",
			permission:    Permissions{},
			expectedBytes: set.NewBits().Bytes(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			keys := Keys{tt.key: &tt.permission}
			permissionBytes := keys[tt.key].ToBytes()
			require.Equal(tt.expectedBytes, permissionBytes)
		})
	}
}

func TestFromBytes(t *testing.T) {
	tests := []struct {
		name               string
		key                string
		permissionBytes    []byte
		expectedPermission *Permissions
	}{
		{
			name:               "key has Read, Allocate, Write",
			key:                "test",
			permissionBytes:    set.NewBits(int(Read), int(Allocate), int(Write)).Bytes(),
			expectedPermission: &Permissions{Read: true, Allocate: true, Write: true},
		},
		{
			name:               "key has no perms",
			key:                "test1",
			permissionBytes:    set.NewBits().Bytes(),
			expectedPermission: &Permissions{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			permissions := FromBytes(tt.permissionBytes)
			require.Equal(tt.expectedPermission, permissions)
		})
	}
}
