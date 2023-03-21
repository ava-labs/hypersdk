// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package client

import (
	"context"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"

	"github.com/ava-labs/hypersdk/requester"
	"github.com/ava-labs/hypersdk/vm"
)

const suggestedFeeCacheRefresh = 10 * time.Second

type Client struct {
	Requester *requester.EndpointRequester

	networkID uint32
	subnetID  ids.ID
	chainID   ids.ID

	lastSuggestedFee time.Time
	unitPrice        uint64
	blockCost        uint64
}

// New creates a new client object.
func New(name string, uri string) *Client {
	req := requester.New(
		fmt.Sprintf("%s%s", uri, vm.Endpoint),
		name,
	)
	return &Client{Requester: req}
}

func (cli *Client) Ping(ctx context.Context) (bool, error) {
	resp := new(vm.PingReply)
	err := cli.Requester.SendRequest(ctx,
		"ping",
		nil,
		resp,
	)
	return resp.Success, err
}

func (cli *Client) Network(ctx context.Context) (uint32, ids.ID, ids.ID, error) {
	if cli.chainID != ids.Empty {
		return cli.networkID, cli.subnetID, cli.chainID, nil
	}

	resp := new(vm.NetworkReply)
	err := cli.Requester.SendRequest(
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

func (cli *Client) Accepted(ctx context.Context) (ids.ID, uint64, int64, error) {
	resp := new(vm.LastAcceptedReply)
	err := cli.Requester.SendRequest(
		ctx,
		"lastAccepted",
		nil,
		resp,
	)
	return resp.BlockID, resp.Height, resp.Timestamp, err
}

func (cli *Client) SuggestedRawFee(ctx context.Context) (uint64, uint64, error) {
	if time.Since(cli.lastSuggestedFee) < suggestedFeeCacheRefresh {
		return cli.unitPrice, cli.blockCost, nil
	}

	resp := new(vm.SuggestedRawFeeReply)
	err := cli.Requester.SendRequest(
		ctx,
		"suggestedRawFee",
		nil,
		resp,
	)
	if err != nil {
		return 0, 0, err
	}
	cli.unitPrice = resp.UnitPrice
	cli.blockCost = resp.BlockCost
	// We update the time last in case there are concurrent requests being
	// processed (we don't want them to get an inconsistent view).
	cli.lastSuggestedFee = time.Now()
	return resp.UnitPrice, resp.BlockCost, nil
}

func (cli *Client) SubmitTx(ctx context.Context, d []byte) (ids.ID, error) {
	resp := new(vm.SubmitTxReply)
	err := cli.Requester.SendRequest(
		ctx,
		"submitTx",
		&vm.SubmitTxArgs{Tx: d},
		resp,
	)
	return resp.TxID, err
}

func (cli *Client) DecisionsPort(ctx context.Context) (uint16, error) {
	resp := new(vm.PortReply)
	err := cli.Requester.SendRequest(
		ctx,
		"decisionsPort",
		nil,
		resp,
	)
	return resp.Port, err
}

func (cli *Client) BlocksPort(ctx context.Context) (uint16, error) {
	resp := new(vm.PortReply)
	err := cli.Requester.SendRequest(
		ctx,
		"blocksPort",
		nil,
		resp,
	)
	return resp.Port, err
}

func (cli *Client) GetWarpSignatures(
	ctx context.Context,
	txID ids.ID,
) (*warp.UnsignedMessage, map[ids.NodeID]*validators.GetValidatorOutput, []*vm.WarpSignature, error) {
	resp := new(vm.GetWarpSignaturesReply)
	if err := cli.Requester.SendRequest(
		ctx,
		"getWarpSignatures",
		&vm.GetWarpSignaturesArgs{TxID: txID},
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
