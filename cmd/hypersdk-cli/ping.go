// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/api/jsonrpc"
)

var endpointPingCmd = &cobra.Command{
	Use:   "ping",
	Short: "Ping the endpoint",
	RunE: func(cmd *cobra.Command, _ []string) error {
		endpoint, err := getConfigValue(cmd, "endpoint", true)
		if err != nil {
			return fmt.Errorf("failed to get endpoint: %w", err)
		}
		client := jsonrpc.NewJSONRPCClient(endpoint)

		success, err := client.Ping(context.Background())
		pingErr := ""
		if err != nil {
			pingErr = err.Error()
		}
		return printValue(cmd, pingResponse{
			PingSucceed: success,
			PingError:   pingErr,
		})
	},
}

type pingResponse struct {
	PingSucceed bool   `json:"ping_succeed"`
	PingError   string `json:"ping_error"`
}

func (r pingResponse) String() string {
	if r.PingSucceed {
		return "✅ Ping succeeded"
	}
	return "❌ Ping failed with error: " + r.PingError
}

func init() {
	rootCmd.AddCommand(endpointPingCmd)
}
