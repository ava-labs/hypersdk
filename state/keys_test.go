// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"crypto/sha256"
	"encoding/binary"
	"math/rand"
	"slices"
	"strconv"
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
	require := require.New(t)
	allPerms := []Permissions{Read, Allocate, Write, None, All}

	tests := []struct {
		perm Permissions
		has  []Permissions
	}{
		{
			perm: Read,
			has:  []Permissions{Read, None},
		},
		{
			perm: Write,
			has:  []Permissions{Read, Write, None},
		},
		{
			perm: Allocate,
			has:  []Permissions{Read, Allocate, None},
		},
		{
			perm: None,
			has:  []Permissions{None},
		},
		{
			perm: All,
			has:  allPerms,
		},
	}

	for _, tt := range tests {
		for _, perm := range allPerms {
			expectedHas := slices.Contains(tt.has, perm)
			require.Equal(expectedHas, tt.perm.Has(perm), "expected %s has %s to be %t", tt.perm, perm, expectedHas)
		}
	}
}

func TestKeysMarshalingSimple(t *testing.T) {
	require := require.New(t)

	// test with read permission.
	keys := Keys{}
	require.True(keys.Add("key1", Read))
	bytes, err := keys.MarshalJSON()
	require.NoError(err)
	require.Equal(`{"6b657931":"read"}`, string(bytes))
	keys = Keys{}
	require.NoError(keys.UnmarshalJSON(bytes))
	require.Len(keys, 1)
	require.Equal(Read, keys["key1"])

	// test with read+write permission.
	keys = Keys{}
	require.True(keys.Add("key2", Read|Write))
	bytes, err = keys.MarshalJSON()
	require.NoError(err)
	require.Equal(`{"6b657932":"write"}`, string(bytes))
	keys = Keys{}
	require.NoError(keys.UnmarshalJSON(bytes))
	require.Len(keys, 1)
	require.Equal(Read|Write, keys["key2"])
}

func (k Keys) compare(k2 Keys) bool {
	if len(k) != len(k2) {
		return false
	}
	for k1, v1 := range k {
		if v2, has := k2[k1]; !has || v1 != v2 {
			return false
		}
	}
	return true
}

func TestKeysMarshalingFuzz(t *testing.T) {
	require := require.New(t)
	rand := rand.New(rand.NewSource(0)) //nolint:gosec
	for fuzzIteration := 0; fuzzIteration < 1000; fuzzIteration++ {
		keys := Keys{}
		for keyIdx := 0; keyIdx < rand.Int()%32; keyIdx++ {
			key := sha256.Sum256(binary.BigEndian.AppendUint64(nil, uint64(keyIdx)))
			keys.Add(string(key[:]), Permissions(rand.Int()%(int(All)+1)))
		}
		bytes, err := keys.MarshalJSON()
		require.NoError(err)
		decodedKeys := Keys{}
		require.NoError(decodedKeys.UnmarshalJSON(bytes))
		require.True(keys.compare(decodedKeys))
	}
}

func TestNewPermissionFromString(t *testing.T) {
	tests := []struct {
		strPerm     string
		perm        Permissions
		expectedErr error
	}{
		{
			strPerm: "read",
			perm:    Read,
		},
		{
			strPerm: "write",
			perm:    Write,
		},
		{
			strPerm: "allocate",
			perm:    Allocate,
		},
		{
			strPerm: "all",
			perm:    All,
		},
		{
			strPerm: "none",
			perm:    None,
		},
		{
			strPerm: "09",
			perm:    Permissions(9),
		},
		{
			strPerm:     "010A",
			expectedErr: errInvalidHexadecimalString,
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			require := require.New(t)
			var perm Permissions
			err := perm.UnmarshalText([]byte(test.strPerm))
			if test.expectedErr != nil {
				require.ErrorIs(err, test.expectedErr)
			} else {
				require.NoError(err)
				require.Equal(test.perm, perm)
			}
		})
	}
}

func TestPermissionStringer(t *testing.T) {
	require := require.New(t)
	require.Equal("read", Read.String())
	require.Equal("write", Write.String())
	require.Equal("allocate", Allocate.String())
	require.Equal("all", All.String())
	require.Equal("none", None.String())
	require.Equal("09", Permissions(9).String())
}

func TestUnmarshalIntoNilKeys(t *testing.T) {
	require := require.New(t)

	keys := Keys{}
	require.True(keys.Add("key1", Read))
	bytes, err := keys.MarshalJSON()
	require.NoError(err)

	var unmarshalledKeys Keys
	require.NoError(unmarshalledKeys.UnmarshalJSON(bytes))
	require.Len(unmarshalledKeys, 1)
}
