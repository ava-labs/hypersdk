// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package test

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ava-labs/avalanchego/ids"
)

func CompileTests(programName string) (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return dir, err
	}
	cmd := exec.Command("cargo", "build", "-p", programName, "--target", "wasm32-unknown-unknown", "--target-dir", "./")
	return cmd.String(), cmd.Run()
}

type Loader struct {
	ProgramName string
}

func (t Loader) GetProgramBytes(_ ids.ID) ([]byte, error) {
	if stringVal, err := CompileTests(t.ProgramName); err != nil {
		return []byte(stringVal), err
	}
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(dir, "/wasm32-unknown-unknown/debug/"+t.ProgramName+".wasm"))
}
