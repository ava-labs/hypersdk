// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package crypto

import (
	"fmt"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/bytecodealliance/wasmtime-go/v14"
	"go.uber.org/zap"

	crypto "github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

const Name = "crypto"

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

	return nil
}

// verifyEDSignature verifies the signature of the message using the provided public key.
// Returns 1 if the signature is valid, 0 otherwise.
func (i *Import) verifyEDSignature(caller *wasmtime.Caller, idPtr int64, msgPtr int64, msgLength int64, signaturePtr int64, pubKeyPtr int64) int32 {
	memory := runtime.NewMemory(runtime.NewExportClient(caller))
	// use crypto/ed25519.Verify from hypersdk
	messageBytes, err := memory.Range(uint64(msgPtr), uint64(msgLength))
	if err != nil {
		i.log.Error("failed to read key from memory",
			zap.Error(err),
		)
		return -1
	}

	signatureBytes, err := memory.Range(uint64(signaturePtr), crypto.SignatureLen)
	if err != nil {
		i.log.Error("failed to read key from memory",
			zap.Error(err),	
		)
		return -1
	}
	sig := crypto.Signature(signatureBytes)

	pubKeyBytes, err := memory.Range(uint64(pubKeyPtr), crypto.PublicKeyLen)
	if err != nil {
		i.log.Error("failed to read key from memory",
			zap.Error(err),
		)
		return -1
	}
	pubKey := crypto.PublicKey(pubKeyBytes)

	result := crypto.Verify(messageBytes, pubKey, sig)
	if result {
		return 1
	}
	return 0
}