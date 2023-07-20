// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/validators"
	autils "github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"golang.org/x/exp/maps"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/requester"
	"github.com/ava-labs/hypersdk/utils"
)

const (
	suggestedFeeCacheRefresh = 10 * time.Second
	waitSleep                = 500 * time.Millisecond
)

type JSONRPCClient struct {
	requester *requester.EndpointRequester

	networkID uint32
	subnetID  ids.ID
	chainID   ids.ID

	lastSuggestedFee time.Time
	unitPrice        uint64
}

func NewJSONRPCClient(uri string) *JSONRPCClient {
	uri = strings.TrimSuffix(uri, "/")
	uri += JSONRPCEndpoint
	req := requester.New(uri, Name)
	return &JSONRPCClient{requester: req}
}

func (cli *JSONRPCClient) Ping(ctx context.Context) (bool, error) {
	resp := new(PingReply)
	err := cli.requester.SendRequest(ctx,
		"ping",
		nil,
		resp,
	)
	return resp.Success, err
}

func (cli *JSONRPCClient) Network(ctx context.Context) (uint32, ids.ID, ids.ID, error) {
	if cli.chainID != ids.Empty {
		return cli.networkID, cli.subnetID, cli.chainID, nil
	}

	resp := new(NetworkReply)
	err := cli.requester.SendRequest(
		ctx,
		"network",
		nil,
		resp,
	)
	if err != nil {
		return 0, ids.Empty, ids.Empty, err
	}
	cli.networkID = resp.NetworkID
	cli.subnetID = resp.SubnetID
	cli.chainID = resp.ChainID
	return resp.NetworkID, resp.SubnetID, resp.ChainID, nil
}

func (cli *JSONRPCClient) Accepted(ctx context.Context) (ids.ID, uint64, int64, error) {
	resp := new(LastAcceptedReply)
	err := cli.requester.SendRequest(
		ctx,
		"lastAccepted",
		nil,
		resp,
	)
	return resp.BlockID, resp.Height, resp.Timestamp, err
}

func (cli *JSONRPCClient) SuggestedRawFee(ctx context.Context) (uint64, error) {
	if time.Since(cli.lastSuggestedFee) < suggestedFeeCacheRefresh {
		return cli.unitPrice, nil
	}

	resp := new(SuggestedRawFeeReply)
	err := cli.requester.SendRequest(
		ctx,
		"suggestedRawFee",
		nil,
		resp,
	)
	if err != nil {
		return 0, err
	}
	cli.unitPrice = resp.UnitPrice
	// We update the time last in case there are concurrent requests being
	// processed (we don't want them to get an inconsistent view).
	cli.lastSuggestedFee = time.Now()
	return resp.UnitPrice, nil
}

func (cli *JSONRPCClient) SubmitTx(ctx context.Context, d []byte) (ids.ID, error) {
	resp := new(SubmitTxReply)
	err := cli.requester.SendRequest(
		ctx,
		"submitTx",
		&SubmitTxArgs{Tx: d},
		resp,
	)
	return resp.TxID, err
}

func (cli *JSONRPCClient) GetWarpSignatures(
	ctx context.Context,
	txID ids.ID,
) (*warp.UnsignedMessage, map[ids.NodeID]*validators.GetValidatorOutput, []*chain.WarpSignature, error) {
	resp := new(GetWarpSignaturesReply)
	if err := cli.requester.SendRequest(
		ctx,
		"getWarpSignatures",
		&GetWarpSignaturesArgs{TxID: txID},
		resp,
	); err != nil {
		return nil, nil, nil, err
	}
	// Ensure message is initialized
	if err := resp.Message.Initialize(); err != nil {
		return nil, nil, nil, err
	}
	m := map[ids.NodeID]*validators.GetValidatorOutput{}
	for _, vdr := range resp.Validators {
		vout := &validators.GetValidatorOutput{
			NodeID: vdr.NodeID,
			Weight: vdr.Weight,
		}
		if len(vdr.PublicKey) > 0 {
			pk, err := bls.PublicKeyFromBytes(vdr.PublicKey)
			if err != nil {
				return nil, nil, nil, err
			}
			vout.PublicKey = pk
		}
		m[vdr.NodeID] = vout
	}
	return resp.Message, m, resp.Signatures, nil
}

type Modifier interface {
	Base(*chain.Base)
}

func (cli *JSONRPCClient) GenerateTransaction(
	ctx context.Context,
	parser chain.Parser,
	wm *warp.Message,
	action chain.Action,
	authFactory chain.AuthFactory,
	modifiers ...Modifier,
) (func(context.Context) error, *chain.Transaction, uint64, error) {
	// Get latest fee info
	unitPrice, err := cli.SuggestedRawFee(ctx)
	if err != nil {
		return nil, nil, 0, err
	}

	return cli.GenerateTransactionManual(parser, wm, action, authFactory, unitPrice, modifiers...)
}

func (cli *JSONRPCClient) GenerateTransactionManual(
	parser chain.Parser,
	wm *warp.Message,
	action chain.Action,
	authFactory chain.AuthFactory,
	unitPrice uint64,
	modifiers ...Modifier,
) (func(context.Context) error, *chain.Transaction, uint64, error) {
	// Construct transaction
	now := time.Now().UnixMilli()
	rules := parser.Rules(now)
	base := &chain.Base{
		Timestamp: utils.UnixRMilli(now, rules.GetValidityWindow()),
		ChainID:   rules.ChainID(),
		UnitPrice: unitPrice, // never pay blockCost
	}

	// Modify gathered data
	for _, m := range modifiers {
		m.Base(base)
	}

	// Ensure warp message is intialized before we marshal it
	if wm != nil {
		if err := wm.Initialize(); err != nil {
			return nil, nil, 0, err
		}
	}

	// Build transaction
	actionRegistry, authRegistry := parser.Registry()
	tx := chain.NewTx(base, wm, action)
	tx, err := tx.Sign(authFactory, actionRegistry, authRegistry)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("%w: failed to sign transaction", err)
	}
	maxUnits, err := tx.MaxUnits(rules)
	if err != nil {
		return nil, nil, 0, err
	}
	fee, err := math.Mul64(maxUnits, unitPrice)
	if err != nil {
		return nil, nil, 0, err
	}

	// Return max fee and transaction for issuance
	return func(ictx context.Context) error {
		_, err := cli.SubmitTx(ictx, tx.Bytes())
		return err
	}, tx, fee, nil
}

func Wait(ctx context.Context, check func(ctx context.Context) (bool, error)) error {
	for ctx.Err() == nil {
		exit, err := check(ctx)
		if err != nil {
			return err
		}
		if exit {
			return nil
		}
		time.Sleep(waitSleep)
	}
	return ctx.Err()
}

// getCanonicalValidatorSet returns the validator set of [subnetID] in a canonical ordering.
// Also returns the total weight on [subnetID].
func getCanonicalValidatorSet(
	_ context.Context,
	vdrSet map[ids.NodeID]*validators.GetValidatorOutput,
) ([]*warp.Validator, uint64, error) {
	var (
		vdrs        = make(map[string]*warp.Validator, len(vdrSet))
		totalWeight uint64
		err         error
	)
	for _, vdr := range vdrSet {
		totalWeight, err = math.Add64(totalWeight, vdr.Weight)
		if err != nil {
			return nil, 0, fmt.Errorf("%w: %v", warp.ErrWeightOverflow, err) //nolint:errorlint
		}

		if vdr.PublicKey == nil {
			fmt.Println("skipping validator because of empty public key", vdr.NodeID)
			continue
		}

		pkBytes := bls.PublicKeyToBytes(vdr.PublicKey)
		uniqueVdr, ok := vdrs[string(pkBytes)]
		if !ok {
			uniqueVdr = &warp.Validator{
				PublicKey:      vdr.PublicKey,
				PublicKeyBytes: pkBytes,
			}
			vdrs[string(pkBytes)] = uniqueVdr
		}

		uniqueVdr.Weight += vdr.Weight // Impossible to overflow here
		uniqueVdr.NodeIDs = append(uniqueVdr.NodeIDs, vdr.NodeID)
	}

	// Sort validators by public key
	vdrList := maps.Values(vdrs)
	autils.Sort(vdrList)
	return vdrList, totalWeight, nil
}

func (cli *JSONRPCClient) GenerateAggregateWarpSignature(
	ctx context.Context,
	txID ids.ID,
) (*warp.Message, uint64, uint64, error) {
	unsignedMessage, validators, signatures, err := cli.GetWarpSignatures(ctx, txID)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("%w: failed to fetch warp signatures", err)
	}

	// Get canonical validator ordering to generate signature bit set
	canonicalValidators, weight, err := getCanonicalValidatorSet(ctx, validators)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("%w: failed to get canonical validator set", err)
	}

	// Generate map of bls.PublicKey => Signature
	signatureMap := map[ids.ID][]byte{}
	for _, signature := range signatures {
		// Convert to hash for easy comparison (could just as easily store the raw
		// public key but that would involve a number of memory copies)
		signatureMap[utils.ToID(signature.PublicKey)] = signature.Signature
	}

	// Generate signature
	signers := set.NewBits()
	var signatureWeight uint64
	orderedSignatures := []*bls.Signature{}
	for i, vdr := range canonicalValidators {
		sig, ok := signatureMap[utils.ToID(vdr.PublicKeyBytes)]
		if !ok {
			continue
		}
		blsSig, err := bls.SignatureFromBytes(sig)
		if err != nil {
			return nil, 0, 0, err
		}
		signers.Add(i)
		signatureWeight += vdr.Weight
		orderedSignatures = append(orderedSignatures, blsSig)
	}
	aggSignature, err := bls.AggregateSignatures(orderedSignatures)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("%w: failed to aggregate signatures", err)
	}
	aggSignatureBytes := bls.SignatureToBytes(aggSignature)
	signature := &warp.BitSetSignature{
		Signers: signers.Bytes(),
	}
	copy(signature.Signature[:], aggSignatureBytes)
	message, err := warp.NewMessage(unsignedMessage, signature)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("%w: failed to generate warp message", err)
	}
	return message, weight, signatureWeight, nil
}
