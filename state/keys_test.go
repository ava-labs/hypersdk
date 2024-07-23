// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"fmt"
	"slices"
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
	allPerms := []Permissions{Read, Allocate, Write, None, All}

	tests := []struct {
		perm    Permissions
		has     []Permissions
		missing []Permissions
	}{
		{
			perm: Read,
			has:  []Permissions{Read, None},
		},
		{
			perm: Allocate,
			has:  []Permissions{Read, Allocate, None},
		},
		{
			perm: Write,
			has:  []Permissions{Read, Write, None},
		},
		{
			perm:    Write,
			missing: []Permissions{Allocate, All},
		},
		{
			perm: None,
			has:  []Permissions{None},
		},
		{
			perm: All,
			has:  allPerms,
		},
		{
			perm:    All,
			missing: []Permissions{},
		},
	}

	for _, tt := range tests {
		if (tt.has == nil) == (tt.missing == nil) {
			require.Fail(t, "please specify either `has` or `missing`")
		}

		for _, perm := range allPerms {
			permString := tt.perm.String()
			ofPermString := perm.String()
			has := tt.perm.Has(perm)

			var name string
			if tt.has != nil {
				if has {
					name = fmt.Sprintf("%s has %s", permString, ofPermString)
				} else {
					name = fmt.Sprintf("%s does not have %s", permString, ofPermString)
				}
			} else {
				if has {
					name = fmt.Sprintf("%s is not missing %s", permString, ofPermString)
				} else {
					name = fmt.Sprintf("%s is missing %s", permString, ofPermString)
				}
			}

			t.Run(name, func(t *testing.T) {
				require := require.New(t)
				if tt.has != nil {
					contains := slices.Contains(tt.has, perm)
					require.Equal(contains, has)
				} else {
					notContains := slices.Contains(tt.missing, perm)
					require.NotEqual(notContains, has)
				}
			})
		}
	}
}
