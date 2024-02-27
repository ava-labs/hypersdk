// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func NewRootCmd() *cobra.Command {
	s := &simulator{}
	cmd := &cobra.Command{
		Use:   "simulator",
		Short: "HyperSDK program VM simulator",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	cobra.EnablePrefixMatching = true
	cmd.CompletionOptions.HiddenDefaultCmd = true
	cmd.DisableAutoGenTag = true
	cmd.SilenceErrors = true
	cmd.SetHelpCommand(&cobra.Command{Hidden: true})
	cmd.PersistentFlags().StringVar(&s.logLevel, "log-level", "info", "log level")

	// initialize simulator vm
	err := s.Init()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// add subcommands
	cmd.AddCommand(
		newRunCmd(s.log, s.db),
		newProgramCmd(s.log, s.db),
		newKeyCmd(s.log, s.db),
	)

	// ensure vm and databases are properly closed on simulator exit
	cobra.OnFinalize(func() {
		if s.vm != nil {
			err := s.vm.Shutdown(cmd.Context())
			if err != nil {
				s.log.Error("simulator vm closed with error",
					zap.Error(err),
				)
			}
		}
	})

	return cmd
}
