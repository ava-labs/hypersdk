// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package scripts

import (
	"embed"
	"errors"
	"io"
	"os/exec"
)

//go:embed *.sh
var scriptsFolder embed.FS

var ErrScriptNotFound = errors.New("script not found")

func RunScript(script string) ([]byte, error) {
	switch script {
	case "run":
		runScript, err := scriptsFolder.Open("run.sh")
		if err != nil {
			return nil, err
		}
		defer runScript.Close()
		scriptContent, err := io.ReadAll(runScript)
		if err != nil {
			return nil, err
		}
		cmd := exec.Command("bash", "-c", string(scriptContent)) // #nosec 204
		return cmd.Output()
	case "stop":
		stopScript, err := scriptsFolder.Open("stop.sh")
		if err != nil {
			return nil, err
		}
		defer stopScript.Close()
		scriptContent, err := io.ReadAll(stopScript)
		if err != nil {
			return nil, err
		}
		cmd := exec.Command("bash", "-c", string(scriptContent)) // #nosec 204
		return cmd.Output()
	default:
		return nil, ErrScriptNotFound
	}
}
