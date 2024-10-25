// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/abi/dynamic"
	"github.com/ava-labs/hypersdk/api/indexer"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
)

var txCmd = &cobra.Command{
	Use:   "tx [action]",
	Short: "Execute a transaction on the chain",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// 1. Decode key
		keyString, err := getConfigValue(cmd, "key", true)
		if err != nil {
			return fmt.Errorf("failed to get key from config: %w", err)
		}
		key, err := privateKeyFromString(keyString)
		if err != nil {
			return fmt.Errorf("failed to decode key: %w", err)
		}

		// 2. create client
		endpoint, err := getConfigValue(cmd, "endpoint", true)
		if err != nil {
			return fmt.Errorf("failed to get endpoint: %w", err)
		}
		client := jsonrpc.NewJSONRPCClient(endpoint)

		// 3. get abi
		abi, err := client.GetABI(ctx)
		if err != nil {
			return fmt.Errorf("failed to get abi: %w", err)
		}
		// 4. get action name from args
		if len(args) == 0 {
			return errors.New("action name is required")
		}
		actionName := args[0]
		_, found := abi.FindActionByName(actionName)
		if !found {
			return fmt.Errorf("failed to find action: %s", actionName)
		}

		typ, found := abi.FindTypeByName(actionName)
		if !found {
			return fmt.Errorf("failed to find type: %s", actionName)
		}

		// 5. create action using kvPairs
		kvPairs, err := fillAction(cmd, typ)
		if err != nil {
			return err
		}

		jsonPayload, err := json.Marshal(kvPairs)
		if err != nil {
			return fmt.Errorf("failed to marshal kvPairs: %w", err)
		}

		actionBytes, err := dynamic.Marshal(abi, actionName, string(jsonPayload))
		if err != nil {
			return fmt.Errorf("failed to marshal action: %w", err)
		}

		_, _, chainID, err := client.Network(ctx)
		if err != nil {
			return fmt.Errorf("failed to get network info: %w", err)
		}

		base := &chain.Base{
			ChainID:   chainID,
			Timestamp: time.Now().Unix()*1000 + 60*1000, // TODO: use utils.UnixRMilli(now, rules.GetValidityWindow())
			MaxFee:    1_000_000,                        // TODO: use chain.EstimateUnits(parser.Rules(time.Now().UnixMilli()), actions, authFactory)
		}

		signedBytes, err := chain.SignRawActionBytesTx(base, append([]byte{1}, actionBytes...), auth.NewED25519Factory(key))
		if err != nil {
			return fmt.Errorf("failed to sign tx: %w", err)
		}

		indexerClient := indexer.NewClient(endpoint)

		expectedTxID, err := client.SubmitTx(ctx, signedBytes)
		if err != nil {
			return fmt.Errorf("failed to send tx: %w", err)
		}

		var getTxResponse indexer.GetTxResponse
		for {
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("context expired while waiting for tx: %w", err)
			}

			getTxResponse, found, err = indexerClient.GetTx(ctx, expectedTxID)
			if err != nil {
				return fmt.Errorf("failed to get tx: %w", err)
			}
			if found {
				break
			}
			time.Sleep(500 * time.Millisecond)
		}

		var resultStruct map[string]interface{}
		if getTxResponse.Success {
			if len(getTxResponse.Outputs) == 1 {
				resultJSON, err := dynamic.UnmarshalOutput(abi, getTxResponse.Outputs[0])
				if err != nil {
					return fmt.Errorf("failed to unmarshal result: %w", err)
				}

				err = json.Unmarshal([]byte(resultJSON), &resultStruct)
				if err != nil {
					return fmt.Errorf("failed to unmarshal result JSON: %w", err)
				}
			} else if len(getTxResponse.Outputs) > 1 {
				return fmt.Errorf("expected 1 output, got %d", len(getTxResponse.Outputs))
			}
		}

		return printValue(cmd, txResponse{
			Result:  resultStruct,
			Success: getTxResponse.Success,
			TxID:    expectedTxID,
			Error:   getTxResponse.ErrorStr,
		})
	},
}

type txResponse struct {
	Result  map[string]interface{} `json:"result"`
	Success bool                   `json:"success"`
	TxID    ids.ID                 `json:"txId"`
	Error   string                 `json:"error"`
}

func (r txResponse) String() string {
	var result strings.Builder
	if r.Success {
		result.WriteString(fmt.Sprintf("✅ Transaction successful (txID: %s)\n", r.TxID))
		if r.Result != nil {
			for key, value := range r.Result {
				jsonValue, err := json.Marshal(value)
				if err != nil {
					jsonValue = []byte(fmt.Sprintf("%v", value))
				}
				result.WriteString(fmt.Sprintf("%s: %s\n", key, string(jsonValue)))
			}
		}
	} else {
		result.WriteString(fmt.Sprintf("❌ Transaction failed (txID: %s): %s\n", r.TxID, r.Error))
	}
	return strings.TrimSpace(result.String())
}

func init() {
	txCmd.Flags().StringToString("data", nil, "Key-value pairs for the action data (e.g., key1=value1,key2=value2)")
	rootCmd.AddCommand(txCmd)
}
