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
