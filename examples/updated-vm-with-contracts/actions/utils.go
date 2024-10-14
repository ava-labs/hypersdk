// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"os"

	"github.com/near/borsh-go"
)

// LoadBytes returns bytes stored at a file [filename].
func LoadBytes(filename string) ([]byte, error) {
	bytes, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

func SerializeArgs(params ...interface{}) ([]byte, error) {
	if len(params) == 0 {
		return nil, nil
	}
	results := make([][]byte, len(params))
	var err error
	for i, param := range params {
		results[i], err = borsh.Serialize(param)
		if err != nil {
			return nil, err
		}
	}
	return Flatten[byte](results...), nil
}

func Flatten[T any](slices ...[]T) []T {
	var size int
	for _, slice := range slices {
		size += len(slice)
	}

	result := make([]T, 0, size)
	for _, slice := range slices {
		result = append(result, slice...)
	}
	return result
}
