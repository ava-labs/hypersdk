package main

import (
	"context"
	"fmt"

	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/spf13/cobra"
)

var endpointPingCmd = &cobra.Command{
	Use:   "ping",
	Short: "Ping the endpoint",
	RunE: func(cmd *cobra.Command, args []string) error {
		endpoint, err := getConfigValue(cmd, "endpoint")
		if err != nil {
			return fmt.Errorf("failed to get endpoint: %w", err)
		}
		client := jsonrpc.NewJSONRPCClient(endpoint)

		success, err := client.Ping(context.Background())
		if err != nil {
			return fmt.Errorf("failed to ping: %w", err)
		}
		return printValue(cmd, pingResponse{
			Success: success,
		})
	},
}

type pingResponse struct {
	Success bool `json:"success"`
}

func (r pingResponse) String() string {
	if r.Success {
		return "✅ Ping succeeded"
	}
	return "❌ Ping failed"
}

func init() {
	rootCmd.AddCommand(endpointPingCmd)
}
