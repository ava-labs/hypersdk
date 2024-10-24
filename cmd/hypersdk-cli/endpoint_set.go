// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/api/jsonrpc"
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

		if err := updateConfig("endpoint", endpoint); err != nil {
			return fmt.Errorf("failed to update config: %w", err)
		}

		client := jsonrpc.NewJSONRPCClient(endpoint)
		success, err := client.Ping(context.Background())
		pingErr := ""
		if err != nil {
			pingErr = err.Error()
		}

		return printValue(cmd, endpointSetCmdResponse{
			Endpoint: endpoint,
			pingResponse: pingResponse{
				PingSucceed: success,
				PingError:   pingErr,
			},
		})
	},
}

type endpointSetCmdResponse struct {
	pingResponse
	Endpoint string `json:"endpoint"`
}

func (r endpointSetCmdResponse) String() string {
	pingStatus := r.pingResponse.String()
	return fmt.Sprintf("Endpoint set to: %s\n%s", r.Endpoint, pingStatus)
}

func init() {
	endpointCmd.AddCommand(endpointSetCmd)
	endpointSetCmd.Flags().String("endpoint", "", "Endpoint URL to set")

	err := endpointSetCmd.MarkFlagRequired("endpoint")
	if err != nil {
		log.Fatalf("failed to mark endpoint flag as required: %s", err)
	}
}
