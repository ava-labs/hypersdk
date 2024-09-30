// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

//nolint:lll
package cmd

import (
	"github.com/spf13/cobra"
)

var prometheusCmd = &cobra.Command{
	Use: "prometheus",
	RunE: func(*cobra.Command, []string) error {
		return ErrMissingSubcommand
	},
}

var generatePrometheusCmd = &cobra.Command{
	Use: "generate",
	RunE: func(_ *cobra.Command, args []string) error {
		return handler.Root().GeneratePrometheus(prometheusBaseURI, prometheusOpenBrowser, startPrometheus, prometheusFile, prometheusData)
	},
}
