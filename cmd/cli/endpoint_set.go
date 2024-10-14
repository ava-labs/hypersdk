package main

import (
	"fmt"

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

		return printValue(cmd, endpointSetCmdResponse{
			Endpoint: endpoint,
		})
	},
}

type endpointSetCmdResponse struct {
	Endpoint    string `json:"endpoint"`
	PingSucceed bool   `json:"ping"`
}

func (r endpointSetCmdResponse) String() string {
	pingStatus := "Ping failed"
	if r.PingSucceed {
		pingStatus = "Ping succeeded"
	}
	return fmt.Sprintf("Endpoint set to: %s\n%s", r.Endpoint, pingStatus)
}

func init() {
	endpointCmd.AddCommand(endpointSetCmd)
	endpointSetCmd.Flags().String("endpoint", "", "Endpoint URL to set")
	endpointSetCmd.MarkFlagRequired("endpoint")
}
