// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package crypto

import (
	"fmt"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/bytecodealliance/wasmtime-go/v14"
	"github.com/near/borsh-go"
	"go.uber.org/zap"

	crypto "github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

const Name = "crypto"

type SignedMessage struct {
	Message []byte
	// signature
	Signature [64]byte
	// public key
	PublicKey [32]byte
}

var _ runtime.Import = &Import{}

type Import struct {
	db         state.Mutable
	log        logging.Logger
	imports    runtime.SupportedImports
	meter      runtime.Meter
	registered bool
}

// New returns a new crypto host module which can perform cryptographic operations.
func New(log logging.Logger, db state.Mutable) *Import {
	return &Import{
		db:  db,
		log: log,
	}
}

func (i *Import) Name() string {
	return Name
}

func (i *Import) Register(link runtime.Link, meter runtime.Meter, imports runtime.SupportedImports) error {
	if i.registered {
		return fmt.Errorf("import module already registered: %q", Name)
	}
	i.imports = imports
	i.meter = meter

	if err := link.FuncWrap(Name, "verify_ed25519", i.verifyEDSignature); err != nil {
		return err
	}
	if err := link.FuncWrap(Name, "batch_verify_ed25519", i.batchVerifyEdSignature); err != nil {
		return err
	}

	return nil
}

// verifyEDSignature verifies the signature of the message using the provided public key.
// Returns 1 if the signature is valid, 0 otherwise.
func (i *Import) verifyEDSignature(caller *wasmtime.Caller, id int64, msg int64) int32 {
	memory := runtime.NewMemory(runtime.NewExportClient(caller))

	signedMessageBytes, err := runtime.SmartPtr(msg).Bytes(memory)
	if err != nil {
		i.log.Error("failed to read key from memory",
			zap.Error(err),
		)
		return -1
	}

	var signedMsg SignedMessage
	err = deserializeParameter(&signedMsg, signedMessageBytes[:])
	if err != nil {
		i.log.Error("failed to deserialize signed message",
			zap.Error(err),
		)
		return -1
	}

	// use crypto/ed25519.Verify from hypersdk
	pubKey := crypto.PublicKey(signedMsg.PublicKey)
	signature := crypto.Signature(signedMsg.Signature)
	result := crypto.Verify(signedMsg.Message[:], pubKey, signature)

	if result {
		return 1
	}
	return 0
}

// batchVerifyEdSignature returns the number of valid signatures in the batch [messages].
func (i *Import) batchVerifyEdSignature(caller *wasmtime.Caller, id int64, messages int64) int32 {
	memory := runtime.NewMemory(runtime.NewExportClient(caller))

	signedMessagesBytes, err := runtime.SmartPtr(messages).Bytes(memory)
	if err != nil {
		i.log.Error("failed to read key from memory",
			zap.Error(err),
		)
		return -1
	}

	var signedMessages []SignedMessage
	err = deserializeParameter(&signedMessages, signedMessagesBytes[:])
	if err != nil {
		i.log.Error("failed to deserialize signed message",
			zap.Error(err),
		)
		return -1
	}

	numVerified := 0
	for _, signedMsg := range signedMessages {
		pubKey := crypto.PublicKey(signedMsg.PublicKey)
		signature := crypto.Signature(signedMsg.Signature)

		result := crypto.Verify(signedMsg.Message[:], pubKey, signature)
		if result {
			numVerified++
		}
	}

	return int32(numVerified)
}

// deserializeParameter deserializes [bytes] using Borsh
func deserializeParameter(obj interface{}, bytes []byte) error {
	return borsh.Deserialize(obj, bytes)
}
