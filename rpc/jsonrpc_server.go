// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"go.uber.org/zap"
)

type JSONRPCServer struct {
	vm VM
}

func NewJSONRPCServer(vm VM) *JSONRPCServer {
	return &JSONRPCServer{vm}
}

type PingReply struct {
	Success bool `json:"success"`
}

func (j *JSONRPCServer) Ping(_ *http.Request, _ *struct{}, reply *PingReply) (err error) {
	j.vm.Logger().Info("ping")
	reply.Success = true
	return nil
}

type NetworkReply struct {
	NetworkID uint32 `json:"networkId"`
	SubnetID  ids.ID `json:"subnetId"`
	ChainID   ids.ID `json:"chainId"`
}

func (j *JSONRPCServer) Network(_ *http.Request, _ *struct{}, reply *NetworkReply) (err error) {
	reply.NetworkID = j.vm.NetworkID()
	reply.SubnetID = j.vm.SubnetID()
	reply.ChainID = j.vm.ChainID()
	return nil
}

type SubmitTxArgs struct {
	Tx []byte `json:"tx"`
}

type SubmitTxReply struct {
	TxID ids.ID `json:"txId"`
}

func (j *JSONRPCServer) SubmitTx(
	req *http.Request,
	args *SubmitTxArgs,
	reply *SubmitTxReply,
) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "JSONRPCServer.SubmitTx")
	defer span.End()

	actionRegistry, authRegistry := j.vm.Registry()
	rtx := codec.NewReader(args.Tx, consts.NetworkSizeLimit) // will likely be much smaller than this
	tx, err := chain.UnmarshalTx(rtx, actionRegistry, authRegistry)
	if err != nil {
		return fmt.Errorf("%w: unable to unmarshal on public service", err)
	}
	if !rtx.Empty() {
		return errors.New("tx has extra bytes")
	}
	if err := tx.AuthAsyncVerify()(); err != nil {
		return err
	}
	txID := tx.ID()
	reply.TxID = txID
	return j.vm.Submit(ctx, false, []*chain.Transaction{tx})[0]
}

type LastAcceptedReply struct {
	Height    uint64 `json:"height"`
	BlockID   ids.ID `json:"blockId"`
	Timestamp int64  `json:"timestamp"`
}

func (j *JSONRPCServer) LastAccepted(_ *http.Request, _ *struct{}, reply *LastAcceptedReply) error {
	blk := j.vm.LastAcceptedBlock()
	reply.Height = blk.Hght
	reply.BlockID = blk.ID()
	reply.Timestamp = blk.Tmstmp
	return nil
}

type SuggestedRawFeeReply struct {
	UnitPrice uint64 `json:"unitPrice"`
	BlockCost uint64 `json:"blockCost"`
}

func (j *JSONRPCServer) SuggestedRawFee(
	req *http.Request,
	_ *struct{},
	reply *SuggestedRawFeeReply,
) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "JSONRPCServer.SuggestedRawFee")
	defer span.End()

	unitPrice, blockCost, err := j.vm.SuggestedFee(ctx)
	if err != nil {
		return err
	}
	reply.UnitPrice = unitPrice
	reply.BlockCost = blockCost
	return nil
}

type GetWarpSignaturesArgs struct {
	TxID ids.ID `json:"txID"`
}

type WarpValidator struct {
	NodeID    ids.NodeID `json:"nodeID"`
	PublicKey []byte     `json:"publicKey"`
	Weight    uint64     `json:"weight"`
}

type GetWarpSignaturesReply struct {
	Validators []*WarpValidator       `json:"validators"`
	Message    *warp.UnsignedMessage  `json:"message"`
	Signatures []*chain.WarpSignature `json:"signatures"`
}

func (j *JSONRPCServer) GetWarpSignatures(
	req *http.Request,
	args *GetWarpSignaturesArgs,
	reply *GetWarpSignaturesReply,
) error {
	_, span := j.vm.Tracer().Start(req.Context(), "JSONRPCServer.GetWarpSignatures")
	defer span.End()

	message, err := j.vm.GetOutgoingWarpMessage(args.TxID)
	if err != nil {
		return err
	}
	if message == nil {
		return ErrMessageMissing
	}

	signatures, err := j.vm.GetWarpSignatures(args.TxID)
	if err != nil {
		return err
	}

	// Ensure we only return valid signatures
	validSignatures := []*chain.WarpSignature{}
	warpValidators := []*WarpValidator{}
	validators, publicKeys := j.vm.CurrentValidators(req.Context())
	for _, sig := range signatures {
		if _, ok := publicKeys[string(sig.PublicKey)]; !ok {
			continue
		}
		validSignatures = append(validSignatures, sig)
	}
	for _, vdr := range validators {
		wv := &WarpValidator{
			NodeID: vdr.NodeID,
			Weight: vdr.Weight,
		}
		if vdr.PublicKey != nil {
			wv.PublicKey = bls.PublicKeyToBytes(vdr.PublicKey)
		}
		warpValidators = append(warpValidators, wv)
	}

	// Optimistically request that we gather signatures if we don't have all of them
	if len(validSignatures) < len(publicKeys) {
		j.vm.Logger().Info(
			"fetching missing signatures",
			zap.Stringer("txID", args.TxID),
			zap.Int(
				"previously collected",
				len(signatures),
			),
			zap.Int("valid", len(validSignatures)),
			zap.Int("current public key count", len(publicKeys)),
		)
		j.vm.GatherSignatures(context.TODO(), args.TxID, message.Bytes())
	}

	reply.Message = message
	reply.Validators = warpValidators
	reply.Signatures = validSignatures
	return nil
}

type GetProofArgs struct {
	StateKeys [][]byte `json:"stateKeys"`
}

type GetProofReply struct {
	Proof []byte `json:"proof"`
}

func (j *JSONRPCServer) GetProof(
	req *http.Request,
	args *GetProofArgs,
	reply *GetProofReply,
) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "JSONRPCServer.GetProof")
	defer span.End()

	// Change value of all keys
	view, err := j.vm.LastAcceptedView()
	if err != nil {
		return err
	}
	preRoot, err := view.GetMerkleRoot(ctx)
	if err != nil {
		return err
	}

	view.SetIntercepter()
	for _, key := range args.StateKeys {
		v, err := view.GetValue(ctx, key)
		if err != nil && err == database.ErrNotFound {
			v = []byte{}
		} else if err != nil {
			return err
		}
		// Make sure we always change the value
		// TODO: make more efficient
		v = append(v, []byte{0x1}...)
		if err := view.Insert(ctx, key, v); err != nil {
			return err
		}

		// TODO: should we also remove things?
	}

	// Extract proof from view interceptor
	if _, err := view.GetMerkleRoot(ctx); err != nil {
		return err
	}
	values, nodes := view.GetInterceptedProofs()
	proof := &chain.Proof{
		Root:       preRoot,
		Proofs:     values,
		PathProofs: nodes,
	}
	// We must marshal because the `Maybe` type is not serialized properly
	c := codec.NewWriter(consts.MaxInt)
	if err := proof.Marshal(c); err != nil {
		return err
	}
	if err := c.Err(); err != nil {
		return err
	}
	reply.Proof = c.Bytes()
	return nil
}
