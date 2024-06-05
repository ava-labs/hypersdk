// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/near/borsh-go"
)

func Into[T any](data []byte) T {
	result := new(T)
	if err := borsh.Deserialize(result, data); err != nil {
		panic(err.Error())
	}
	return *result
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

func CompileTest(programName string) error {
	cmd := exec.Command("cargo", "build", "-p", programName, "--target", "wasm32-unknown-unknown", "--target-dir", "./")
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

type Loader struct {
	ProgramName string
}

func (t Loader) GetProgramBytes(_ context.Context, _ ids.ID) ([]byte, error) {
	return load(t.ProgramName)
}

type MapLoader struct {
	Programs map[ids.ID]string
}

func (t MapLoader) GetProgramBytes(_ context.Context, id ids.ID) ([]byte, error) {
	programName, ok := t.Programs[id]
	if !ok {
		return nil, errors.New("program not found")
	}
	return load(programName)
}

func load(programName string) ([]byte, error) {
	if err := CompileTest(programName); err != nil {
		return nil, err
	}
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(dir, "/wasm32-unknown-unknown/debug/"+programName+".wasm"))
}
