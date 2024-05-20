// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ava-labs/avalanchego/ids"
)

func CompileTest(tmpDir, programName string) error {
	cmd := exec.Command("cargo", "build", "-p", programName, "--target", "wasm32-unknown-unknown", "--target-dir", tmpDir)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		fmt.Println(fmt.Sprint(err) + ": " + stderr.String())
		return err
	}
	fmt.Println("Result: " + out.String())
	return nil
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
