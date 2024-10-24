package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/ava-labs/hypersdk/abi/dynamic"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/api/ws"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/spf13/cobra"
)

var txCmd = &cobra.Command{
	Use:   "tx [action]",
	Short: "Execute a transaction on the chain",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		//1. figure out sender address
		keyString, err := getConfigValue(cmd, "key")
		if err != nil {
			return fmt.Errorf("failed to get key from config: %w", err)
		}
		key, err := privateKeyFromString(keyString)
		if err != nil {
			return fmt.Errorf("failed to decode key: %w", err)
		}

		//2. create client
		endpoint, err := getConfigValue(cmd, "endpoint")
		if err != nil {
			return fmt.Errorf("failed to get endpoint: %w", err)
		}
		client := jsonrpc.NewJSONRPCClient(endpoint)

		//3. get abi
		abi, err := client.GetABI(ctx)
		if err != nil {
			return fmt.Errorf("failed to get abi: %w", err)
		}

		//4. get action name from args
		if len(args) == 0 {
			return fmt.Errorf("action name is required")
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

		//5. create action using kvPairs
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
			Timestamp: time.Now().Unix()*1000 + 59000,
			MaxFee:    1_000_000,
		}

		signedBytes, err := SignTxManually([][]byte{actionBytes}, base, key)
		if err != nil {
			return fmt.Errorf("failed to sign tx: %w", err)
		}

		// signedBytesOverrideHex := "00000192bcaf8728bc268cf27206903fbd7975e22f25a6190b8b7666858744aa8803c5d3e4997a56000000000000c0940200010203040000000000000000000000000000000000000000000000000000000000000000003b9aca000000000b74657374206d656d6f20310005060708000000000000000000000000000000000000000000000000000000000000000000773594000000000b74657374206d656d6f2032001b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa70d9c361e5e0b495cbf2c5a2a1b68a71f01cba00062ee417ee8ea4652b40a8f953695db3ed217be20d202fe1ff053b7ad3e557c92dc3125d5c4c953673ed62a04"

		// if signedBytesOverrideHex != "" {
		// 	signedBytes, err = hex.DecodeString(signedBytesOverrideHex)
		// 	if err != nil {
		// 		return fmt.Errorf("failed to decode signedBytesOverrideHex: %w", err)
		// 	}
		// }

		wsClient, err := ws.NewWebSocketClient(endpoint, 10*time.Second, 100, 1000000)
		if err != nil {
			return fmt.Errorf("failed to create ws client: %w", err)
		}

		if err := wsClient.RegisterRawTx(signedBytes); err != nil {
			return fmt.Errorf("failed to register tx: %w", err)
		}

		expectedTxID := utils.ToID(signedBytes)

		fmt.Println("expectedTxID", expectedTxID)

		// Listen for the transaction result
		var result *chain.Result
		for {
			txID, txErr, txResult, err := wsClient.ListenTx(ctx)
			fmt.Println("txID", txID)
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
			log.Printf("Skipping unexpected transaction: %s\n", txID)
		}

		// Check transaction result
		if !result.Success {
			return fmt.Errorf("transaction failed: %s", result.Error)
		}

		//TODO: decode outputs
		//TODO: print outputs
		//TODO: print errors as json/stringer

		return nil
	},
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
	authBytes := append([]byte{auth.GetTypeID()}, p.Bytes()...)

	// Combine everything into final signed transaction
	signedBytes := append(unsignedBytes, authBytes...)
	return signedBytes, nil
}
