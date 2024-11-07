// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package test

import (
	"bytes"
	"fmt"
	"os/exec"

	"github.com/near/borsh-go"
)

func CompileTest(contractName string) error {
	cmd := exec.Command("cargo", "rustc", "-p", contractName, "--release", "--target-dir=./target", "--target=wasm32-unknown-unknown", "--crate-type=cdylib")
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		fmt.Println(fmt.Sprint(err) + ": " + stderr.String())
		return err
	}
	return nil
}

func SerializeParams(params ...interface{}) []byte {
	if len(params) == 0 {
		return nil
	}
	results := make([][]byte, len(params))
	var err error
	for i, param := range params {
		results[i], err = borsh.Serialize(param)
		if err != nil {
			return nil
		}
	}
	return Flatten[byte](results...)
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
