package tests

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ava-labs/hypersdk/abi"
	"github.com/ava-labs/hypersdk/abi/dynamic"
	"github.com/ava-labs/hypersdk/crypto/ed25519"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/vm"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/api/jsonrpc"
)

func TestSignTx(t *testing.T) {
	keyBytes, err := codec.LoadHex("0x323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7", -1)
	require.NoError(t, err)

	key := ed25519.PrivateKey(keyBytes)

	factory := auth.NewED25519Factory(key)

	action1 := actions.Transfer{
		To:    [33]byte{1, 2, 3, 4},
		Value: 1000000000,
		Memo:  []byte("test memo 1"),
	}

	action2 := actions.Transfer{
		To:    [33]byte{5, 6, 7, 8},
		Value: 2000000000,
		Memo:  []byte("test memo 2"),
	}

	hyperVMRPC := vm.NewJSONRPCClient("http://localhost:9650/ext/bc/morpheusvm/")
	hyperSDKRPC := jsonrpc.NewJSONRPCClient("http://localhost:9650/ext/bc/morpheusvm/")

	parser, err := hyperVMRPC.Parser(context.Background())
	require.NoError(t, err)

	_, tx, _, err := hyperSDKRPC.GenerateTransaction(context.Background(), parser, []chain.Action{&action1, &action2}, factory)
	require.NoError(t, err)

	signedBytes := tx.Bytes()

	abi, err := hyperSDKRPC.GetABI(context.Background())
	require.NoError(t, err)

	// Use the manual signing function
	manuallySignedBytes, err := SignTxManually([]chain.Action{&action1, &action2}, "Transfer", tx.Base, abi, key)
	require.NoError(t, err)

	// Compare results
	require.Equal(t, signedBytes, manuallySignedBytes, "signed bytes do not match")
}

// base := &chain.Base{
// 	ChainID:   chainID,
// 	Timestamp: time.Now().Unix()*1000 + 60000,
// 	MaxFee:    1_000_000,
// }

func SignTxManually(actions []chain.Action, actionType string, base *chain.Base, abi abi.ABI, privateKey ed25519.PrivateKey) ([]byte, error) {
	// Create auth factory
	factory := auth.NewED25519Factory(privateKey)

	// Marshal base
	p := codec.NewWriter(base.Size(), consts.NetworkSizeLimit)
	base.Marshal(p)
	baseBytes := p.Bytes()

	// Build unsigned bytes starting with base and number of actions
	unsignedBytes := make([]byte, 0)
	unsignedBytes = append(unsignedBytes, baseBytes...)
	unsignedBytes = append(unsignedBytes, byte(len(actions))) // Number of actions

	// Marshal and append each action
	for _, action := range actions {
		jsonPayload, err := json.Marshal(action)
		if err != nil {
			return nil, err
		}

		actionBytes, err := dynamic.Marshal(abi, actionType, string(jsonPayload))
		if err != nil {
			return nil, err
		}

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
