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
)

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
	programs map[ids.ID]string
}

func (t MapLoader) GetProgramBytes(_ context.Context, id ids.ID) ([]byte, error) {
	programName, ok := t.programs[id]
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
