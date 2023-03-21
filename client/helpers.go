// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package client

import (
	"context"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/validators"
	autils "github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/utils"
	"golang.org/x/exp/maps"
)

const waitSleep = 500 * time.Millisecond

type Modifier interface {
	Base(*chain.Base)
}

func (cli *Client) GenerateTransaction(
	ctx context.Context,
	parser chain.Parser,
	wm *warp.Message,
	action chain.Action,
	authFactory chain.AuthFactory,
	modifiers ...Modifier,
) (func(context.Context) error, *chain.Transaction, uint64, error) {
	// Get latest fee info
	unitPrice, _, err := cli.SuggestedRawFee(ctx)
	if err != nil {
		return nil, nil, 0, err
	}

	// Construct transaction
	now := time.Now().Unix()
	rules := parser.Rules(now)
	base := &chain.Base{
		Timestamp: now + rules.GetValidityWindow(),
		ChainID:   parser.ChainID(),
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
	tx, err = tx.Sign(authFactory, actionRegistry, authRegistry)
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

func (cli *Client) GenerateAggregateWarpSignature(
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
