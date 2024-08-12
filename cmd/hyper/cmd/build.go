// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

type buildOptions struct {
	buildDirectory string
}

var buildOpts buildOptions

// buildCmd represents the build command
var buildCmd = &cobra.Command{
	Use:   "build PROJECT_ROOT PROJECT_NAME",
	Short: "build PROJECT_NAME go binary",
	Long: `build command will build the project binary from the PROJECT_ROOT/cmd/PROJECT_NAME
	into the PROJECT_ROOT/build/<build_path>. build_path is either the PROJECT_NAME or overrided by build-directory flag.`,
	Args: func(cmd *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(2)(cmd, args); err != nil {
			return err
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		projectRoot := args[0]
		projectName := args[1]
		buildDir := projectName
		if buildOpts.buildDirectory != "" {
			buildDir = buildOpts.buildDirectory
		}
		binaryPath := filepath.Join(projectRoot, "build", buildDir)
		projectPath, err := filepath.Abs(filepath.Join(projectRoot, "cmd", projectName))
		if err != nil {
			cmd.PrintErrf("find project name absolute path failed: %s", err.Error())
			os.Exit(1)
		}

		gocmd := exec.CommandContext(context.Background(), "go", "build", "-o", binaryPath, projectPath)
		gocmd.Env = append(os.Environ(), "CGO_CFLAGS=-O -D__BLST_PORTABLE__")
		gocmd.Stdout = cmd.OutOrStdout()
		gocmd.Stderr = cmd.OutOrStderr()

		cmd.Printf("building %s in %s\n", projectName, binaryPath)
		cmd.Printf("command execution: %s\n", gocmd.String())
		err = gocmd.Run()
		if err != nil {
			cmd.PrintErrf("Failed to build project %s: %s\n", projectName, err.Error())
			os.Exit(1)
		}
		cmd.Printf("build: success")
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)

	buildCmd.Flags().StringVarP(&buildOpts.buildDirectory, "build-directory", "", "", "set a specific build directory instead of using project name")
}
