package main

import (
	"context"
	"fmt"

	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/spf13/cobra"
)

var endpointSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set the endpoint URL",
	RunE: func(cmd *cobra.Command, args []string) error {
		endpoint, err := cmd.Flags().GetString("endpoint")
		if err != nil {
			return fmt.Errorf("failed to get endpoint flag: %w", err)
		}

		if endpoint == "" {
			return fmt.Errorf("endpoint is required")
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
	endpointSetCmd.MarkFlagRequired("endpoint")
}
