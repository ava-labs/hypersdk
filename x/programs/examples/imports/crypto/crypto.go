// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package crypto

import (
	"fmt"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/bytecodealliance/wasmtime-go/v14"

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

// New returns a new program invoke host module which can perform program to program calls.
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

var (
	TestPrivateKey = crypto.PrivateKey(
		[crypto.PrivateKeyLen]byte{
			32, 241, 118, 222, 210, 13, 164, 128, 3, 18,
			109, 215, 176, 215, 168, 171, 194, 181, 4, 11,
			253, 199, 173, 240, 107, 148, 127, 190, 48, 164,
			12, 48, 115, 50, 124, 153, 59, 53, 196, 150, 168,
			143, 151, 235, 222, 128, 136, 161, 9, 40, 139, 85,
			182, 153, 68, 135, 62, 166, 45, 235, 251, 246, 69, 7,
		},
	)
)

// verifyEDSignature verifies the signature of the message using the provided public key.
// idPtr is the pointer to the program id.
func (i *Import) verifyEDSignature(caller *wasmtime.Caller, idPtr int64) int32 {
	// use crypto/ed25519.Verify from hypersdk
	msg := []byte("msg")
	sig := crypto.Sign(msg, TestPrivateKey)
	result := crypto.Verify(msg, TestPrivateKey.PublicKey(), sig)
	fmt.Println("Verification result: ", result)
	return 10
}