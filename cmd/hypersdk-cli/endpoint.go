// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var endpointCmd = &cobra.Command{
	Use:   "endpoint",
	Short: "Manage endpoint",
	RunE: func(cmd *cobra.Command, _ []string) error {
		endpoint, err := getConfigValue(cmd, "endpoint", true)
		if err != nil {
			return fmt.Errorf("failed to get endpoint: %w", err)
		}
		return printValue(cmd, endpointCmdResponse{
			Endpoint: endpoint,
		})
	},
}

type endpointCmdResponse struct {
	Endpoint string `json:"endpoint"`
}

func (r endpointCmdResponse) String() string {
	return r.Endpoint
}

func init() {
	rootCmd.AddCommand(endpointCmd)
}
