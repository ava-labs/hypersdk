// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"errors"
	"fmt"
	"log"

	"github.com/spf13/cobra"
)

var endpointSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set the endpoint URL",
	RunE: func(cmd *cobra.Command, _ []string) error {
		endpoint, err := cmd.Flags().GetString("endpoint")
		if err != nil {
			return fmt.Errorf("failed to get endpoint flag: %w", err)
		}

		if endpoint == "" {
			return errors.New("endpoint is required")
		}

		if err := setConfigValue("endpoint", endpoint); err != nil {
			return fmt.Errorf("failed to update config: %w", err)
		}

		return printValue(cmd, endpointSetCmdResponse{
			Endpoint: endpoint,
		})
	},
}

type endpointSetCmdResponse struct {
	Endpoint string `json:"endpoint"`
}

func (r endpointSetCmdResponse) String() string {
	return "Endpoint set to: " + r.Endpoint
}

func init() {
	endpointCmd.AddCommand(endpointSetCmd)
	endpointSetCmd.Flags().String("endpoint", "", "Endpoint URL to set")

	err := endpointSetCmd.MarkFlagRequired("endpoint")
	if err != nil {
		log.Fatalf("failed to mark endpoint flag as required: %s", err)
	}
}
