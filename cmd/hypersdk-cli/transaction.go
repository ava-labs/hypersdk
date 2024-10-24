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
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/api/ws"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/utils"
)

var txCmd = &cobra.Command{
	Use:   "tx [action]",
	Short: "Execute a transaction on the chain",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// 1. Decode key
		keyString, err := getConfigValue(cmd, "key")
		if err != nil {
			return fmt.Errorf("failed to get key from config: %w", err)
		}
		key, err := privateKeyFromString(keyString)
		if err != nil {
			return fmt.Errorf("failed to decode key: %w", err)
		}

		// 2. create client
		endpoint, err := getConfigValue(cmd, "endpoint")
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
			Timestamp: time.Now().Unix()*1000 + 60*1000,
			MaxFee:    1_000_000,
		}

		signedBytes, err := SignTxManually([][]byte{actionBytes}, base, key)
		if err != nil {
			return fmt.Errorf("failed to sign tx: %w", err)
		}

		wsClient, err := ws.NewWebSocketClient(endpoint, 10*time.Second, 100, 1000000)
		if err != nil {
			return fmt.Errorf("failed to create ws client: %w", err)
		}

		if err := wsClient.RegisterRawTx(signedBytes); err != nil {
			return fmt.Errorf("failed to register tx: %w", err)
		}

		expectedTxID := utils.ToID(signedBytes)
		// Listen for the transaction result
		var result *chain.Result
		for {
			txID, txErr, txResult, err := wsClient.ListenTx(ctx)
			if err != nil {
				if ctx.Err() == context.DeadlineExceeded {
					return fmt.Errorf("failed to listen for transaction: %w", ctx.Err())
				}
				return fmt.Errorf("failed to listen for transaction: %w", err)
			}
			if txErr != nil {
				return txErr
			}
			if txID == expectedTxID {
				result = txResult
				break
			}
		}

		var resultStruct map[string]interface{}
		if result.Success {
			if len(result.Outputs) == 1 {
				resultJSON, err := dynamic.UnmarshalOutput(abi, result.Outputs[0])
				if err != nil {
					return fmt.Errorf("failed to unmarshal result: %w", err)
				}

				err = json.Unmarshal([]byte(resultJSON), &resultStruct)
				if err != nil {
					return fmt.Errorf("failed to unmarshal result JSON: %w", err)
				}
			} else if len(result.Outputs) > 1 {
				return fmt.Errorf("expected 1 output, got %d", len(result.Outputs))
			}
		}

		return printValue(cmd, txResponse{
			Result:  resultStruct,
			Success: result.Success,
			TxID:    expectedTxID,
			Error:   string(result.Error),
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
		result.WriteString(fmt.Sprintf("✅ Transaction successful (txID: %s)\n\n", r.TxID))
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

func SignTxManually(actionsTxBytes [][]byte, base *chain.Base, privateKey ed25519.PrivateKey) ([]byte, error) {
	// Create auth factory
	factory := auth.NewED25519Factory(privateKey)

	// Marshal base
	p := codec.NewWriter(base.Size(), consts.NetworkSizeLimit)
	base.Marshal(p)
	baseBytes := p.Bytes()

	// Build unsigned bytes starting with base and number of actions
	unsignedBytes := make([]byte, 0)
	unsignedBytes = append(unsignedBytes, baseBytes...)
	unsignedBytes = append(unsignedBytes, byte(len(actionsTxBytes))) // Number of actions

	// Append each action's bytes
	for _, actionBytes := range actionsTxBytes {
		unsignedBytes = append(unsignedBytes, actionBytes...)
	}

	// Sign the transaction
	auth, err := factory.Sign(unsignedBytes)
	if err != nil {
		return nil, err
	}
	// Marshal auth
	p = codec.NewWriter(auth.Size(), consts.NetworkSizeLimit)
	auth.Marshal(p)
	authBytes := []byte{auth.GetTypeID()}
	authBytes = append(authBytes, p.Bytes()...)

	// Combine everything into final signed transaction
	//nolint:gocritic //append is fine here
	signedBytes := append(unsignedBytes, authBytes...)
	return signedBytes, nil
}
