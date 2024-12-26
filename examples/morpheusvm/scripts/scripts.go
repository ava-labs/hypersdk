// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package scripts

import (
	"embed"
	"errors"
	"io"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/utils"
)

var (
	//go:embed *.sh
	scriptsFolder embed.FS

	ScriptsMapping    map[string]*cobra.Command
	ErrScriptNotFound = errors.New("script not found")
)

func init() {
	ScriptsMapping = make(map[string]*cobra.Command)
	scriptNames, err := getAllScripts()
	if err != nil {
		panic("unable to get script names")
	}
	for _, n := range scriptNames {
		putScriptCommand(n)
	}
}

func putScriptCommand(n string) {
	cmd := &cobra.Command{
		Use: n,
		RunE: func(cmd *cobra.Command, args []string) error {
			output, err := RunScript(n)
			utils.Outf(string(output))
			return err
		},
	}
	ScriptsMapping[n] = cmd
}

func getAllScripts() ([]string, error) {
	dirContents, err := scriptsFolder.ReadDir(".")
	if err != nil {
		return nil, err
	}
	scriptNames := make([]string, len(dirContents))
	for _, v := range dirContents {
		scriptName := strings.TrimSuffix(v.Name(), ".sh")
		scriptNames = append(scriptNames, scriptName)
	}

	return scriptNames, nil
}

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
