// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package scripts

import (
	"embed"
	"errors"
	"io"
	"os/exec"
	"strings"
)

//go:embed *.sh
var scriptsFolder embed.FS

var ErrScriptNotFound = errors.New("script not found")

func RunScript(script string) ([]byte, error) {
	var shPath strings.Builder
	if _, err := shPath.WriteString(script); err != nil {
		return nil, err
	}
	if _, err := shPath.WriteString(".sh"); err != nil {
		return nil, err
	}

	scriptFile, err := scriptsFolder.Open(shPath.String())
	if err != nil {
		return nil, err
	}
	defer scriptFile.Close()
	scriptContent, err := io.ReadAll(scriptFile)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("bash", "-c", string(scriptContent)) // #nosec 204
	return cmd.Output()
}
