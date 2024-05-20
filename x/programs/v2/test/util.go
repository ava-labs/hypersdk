// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package test

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ava-labs/avalanchego/ids"
)

func CompileTest(TmpDir, programName string) error {
	return exec.Command("cargo", "build", "-p", programName, "--target", "wasm32-unknown-unknown", "--target-dir", TmpDir).Run()
}

type Loader struct {
	ProgramName string
	TmpDir      string
}

func (t Loader) GetProgramBytes(_ ids.ID) ([]byte, error) {
	if err := CompileTest(t.TmpDir, t.ProgramName); err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(t.TmpDir, "/wasm32-unknown-unknown/debug/"+t.ProgramName+".wasm"))
}
