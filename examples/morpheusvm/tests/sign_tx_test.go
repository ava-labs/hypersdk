package tests

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

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

	action := actions.Transfer{
		To:    [33]byte{1, 2, 3, 4},
		Value: 1000000000,
		Memo:  []byte("test memo"),
	}

	hyperVMRPC := vm.NewJSONRPCClient("http://localhost:9650/ext/bc/morpheusvm/")
	hyperSDKRPC := jsonrpc.NewJSONRPCClient("http://localhost:9650/ext/bc/morpheusvm/")

	parser, err := hyperVMRPC.Parser(context.Background())
	require.NoError(t, err)

	_, tx, _, err := hyperSDKRPC.GenerateTransaction(context.Background(), parser, []chain.Action{&action}, factory)
	require.NoError(t, err)

	unsignedBytes, err := tx.UnsignedBytes()
	require.NoError(t, err)
	fmt.Println("unsignedBytes", hex.EncodeToString(unsignedBytes))

	signedBytes := tx.Bytes()
	fmt.Println("signedBytes", hex.EncodeToString(signedBytes))

	base := tx.Base
	p := codec.NewWriter(base.Size(), consts.NetworkSizeLimit)
	base.Marshal(p)
	baseBytes := p.Bytes()
	fmt.Println("baseBytes", hex.EncodeToString(baseBytes))

	abi, err := hyperSDKRPC.GetABI(context.Background())
	require.NoError(t, err)

	jsonPayload, err := json.Marshal(action)
	require.NoError(t, err)

	actionBytes, err := dynamic.Marshal(abi, "Transfer", string(jsonPayload))
	require.NoError(t, err)

	fmt.Println("actionBytes", hex.EncodeToString(actionBytes))

	// Concatenate baseBytes, action count (0x01), and actionBytes
	actualUnsignedBytes := make([]byte, 0, len(baseBytes)+1+len(actionBytes))
	actualUnsignedBytes = append(actualUnsignedBytes, baseBytes...)
	actualUnsignedBytes = append(actualUnsignedBytes, 0x01) // Single action
	actualUnsignedBytes = append(actualUnsignedBytes, actionBytes...)

	// Compare with original unsigned bytes
	require.Equal(t, unsignedBytes, actualUnsignedBytes, "unsigned bytes do not match")

	actualAuth, err := factory.Sign(actualUnsignedBytes) // Sign the actualUnsignedBytes with the factory
	require.NoError(t, err)

	signedBytesDebugStr := hex.EncodeToString(signedBytes)
	actualUnsignedBytesStr := hex.EncodeToString(actualUnsignedBytes)
	fmt.Printf("signedBytes: %s\n", signedBytesDebugStr)

	signedBytesDebugStr = strings.Replace(signedBytesDebugStr, actualUnsignedBytesStr, "_actualUnsignedBytesStr_", -1)
	fmt.Printf("signedBytes: %s\n", signedBytesDebugStr)

	p = codec.NewWriter(actualAuth.Size(), consts.NetworkSizeLimit)
	actualAuth.Marshal(p)
	actualAuthBytes := append([]byte{actualAuth.GetTypeID()}, p.Bytes()...)

	actualAuthBytesStr := hex.EncodeToString(actualAuthBytes)
	signedBytesDebugStr = strings.Replace(signedBytesDebugStr, actualAuthBytesStr, "_actualAuthBytesStr_", -1)
	fmt.Printf("signedBytes: %s\n", signedBytesDebugStr)

	actualTxBytes := append(actualUnsignedBytes, actualAuthBytes...)
	require.Equal(t, signedBytes, actualTxBytes, "signed bytes do not match")
}
