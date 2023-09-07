// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

//nolint:lll
package cmd

import (
	"context"
	"errors"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/tokenvm/actions"
	trpc "github.com/ava-labs/hypersdk/examples/tokenvm/rpc"
	"github.com/ava-labs/hypersdk/examples/tokenvm/utils"
	"github.com/ava-labs/hypersdk/rpc"
	hutils "github.com/ava-labs/hypersdk/utils"
	"github.com/spf13/cobra"
)

var actionCmd = &cobra.Command{
	Use: "action",
	RunE: func(*cobra.Command, []string) error {
		return ErrMissingSubcommand
	},
}

var transferCmd = &cobra.Command{
	Use: "transfer",
	RunE: func(*cobra.Command, []string) error {
		ctx := context.Background()
		_, priv, factory, cli, tcli, err := handler.DefaultActor()
		if err != nil {
			return err
		}

		// Select token to send
		assetID, err := handler.Root().PromptAsset("assetID", true)
		if err != nil {
			return err
		}
		balance, _, err := handler.GetAssetInfo(ctx, tcli, priv.PublicKey(), assetID, true)
		if balance == 0 || err != nil {
			return err
		}

		// Select recipient
		recipient, err := handler.Root().PromptAddress("recipient")
		if err != nil {
			return err
		}

		// Select amount
		amount, err := handler.Root().PromptAmount("amount", assetID, balance, nil)
		if err != nil {
			return err
		}

		// Confirm action
		cont, err := handler.Root().PromptContinue()
		if !cont || err != nil {
			return err
		}

		// Generate transaction
		_, _, err = sendAndWait(ctx, nil, &actions.Transfer{
			To:    recipient,
			Asset: assetID,
			Value: amount,
		}, cli, tcli, factory, true)
		return err
	},
}

var createAssetCmd = &cobra.Command{
	Use: "create-asset",
	RunE: func(*cobra.Command, []string) error {
		ctx := context.Background()
		_, _, factory, cli, tcli, err := handler.DefaultActor()
		if err != nil {
			return err
		}

		// Add metadata to token
		metadata, err := handler.Root().PromptString("metadata (can be changed later)", 0, actions.MaxMetadataSize)
		if err != nil {
			return err
		}

		// Confirm action
		cont, err := handler.Root().PromptContinue()
		if !cont || err != nil {
			return err
		}

		// Generate transaction
		_, _, err = sendAndWait(ctx, nil, &actions.CreateAsset{
			Metadata: []byte(metadata),
		}, cli, tcli, factory, true)
		return err
	},
}

var mintAssetCmd = &cobra.Command{
	Use: "mint-asset",
	RunE: func(*cobra.Command, []string) error {
		ctx := context.Background()
		_, priv, factory, cli, tcli, err := handler.DefaultActor()
		if err != nil {
			return err
		}

		// Select token to mint
		assetID, err := handler.Root().PromptAsset("assetID", false)
		if err != nil {
			return err
		}
		exists, metadata, supply, owner, warp, err := tcli.Asset(ctx, assetID)
		if err != nil {
			return err
		}
		if !exists {
			hutils.Outf("{{red}}%s does not exist{{/}}\n", assetID)
			hutils.Outf("{{red}}exiting...{{/}}\n")
			return nil
		}
		if warp {
			hutils.Outf("{{red}}cannot mint a warped asset{{/}}\n", assetID)
			hutils.Outf("{{red}}exiting...{{/}}\n")
			return nil
		}
		if owner != utils.Address(priv.PublicKey()) {
			hutils.Outf("{{red}}%s is the owner of %s, you are not{{/}}\n", owner, assetID)
			hutils.Outf("{{red}}exiting...{{/}}\n")
			return nil
		}
		hutils.Outf(
			"{{yellow}}metadata:{{/}} %s {{yellow}}supply:{{/}} %d\n",
			string(metadata),
			supply,
		)

		// Select recipient
		recipient, err := handler.Root().PromptAddress("recipient")
		if err != nil {
			return err
		}

		// Select amount
		amount, err := handler.Root().PromptAmount("amount", assetID, consts.MaxUint64-supply, nil)
		if err != nil {
			return err
		}

		// Confirm action
		cont, err := handler.Root().PromptContinue()
		if !cont || err != nil {
			return err
		}

		// Generate transaction
		_, _, err = sendAndWait(ctx, nil, &actions.MintAsset{
			Asset: assetID,
			To:    recipient,
			Value: amount,
		}, cli, tcli, factory, true)
		return err
	},
}

var closeOrderCmd = &cobra.Command{
	Use: "close-order",
	RunE: func(*cobra.Command, []string) error {
		ctx := context.Background()
		_, _, factory, cli, tcli, err := handler.DefaultActor()
		if err != nil {
			return err
		}

		// Select inbound token
		orderID, err := handler.Root().PromptID("orderID")
		if err != nil {
			return err
		}

		// Select outbound token
		outAssetID, err := handler.Root().PromptAsset("out assetID", true)
		if err != nil {
			return err
		}

		// Confirm action
		cont, err := handler.Root().PromptContinue()
		if !cont || err != nil {
			return err
		}

		// Generate transaction
		_, _, err = sendAndWait(ctx, nil, &actions.CloseOrder{
			Order: orderID,
			Out:   outAssetID,
		}, cli, tcli, factory, true)
		return err
	},
}

var createOrderCmd = &cobra.Command{
	Use: "create-order",
	RunE: func(*cobra.Command, []string) error {
		ctx := context.Background()
		_, priv, factory, cli, tcli, err := handler.DefaultActor()
		if err != nil {
			return err
		}

		// Select inbound token
		inAssetID, err := handler.Root().PromptAsset("in assetID", true)
		if err != nil {
			return err
		}
		if inAssetID != ids.Empty {
			exists, metadata, supply, _, warp, err := tcli.Asset(ctx, inAssetID)
			if err != nil {
				return err
			}
			if !exists {
				hutils.Outf("{{red}}%s does not exist{{/}}\n", inAssetID)
				hutils.Outf("{{red}}exiting...{{/}}\n")
				return nil
			}
			hutils.Outf(
				"{{yellow}}metadata:{{/}} %s {{yellow}}supply:{{/}} %d {{yellow}}warp:{{/}} %t\n",
				string(metadata),
				supply,
				warp,
			)
		}

		// Select in tick
		inTick, err := handler.Root().PromptAmount("in tick", inAssetID, consts.MaxUint64, nil)
		if err != nil {
			return err
		}

		// Select outbound token
		outAssetID, err := handler.Root().PromptAsset("out assetID", true)
		if err != nil {
			return err
		}
		balance, _, err := handler.GetAssetInfo(ctx, tcli, priv.PublicKey(), outAssetID, true)
		if balance == 0 || err != nil {
			return err
		}

		// Select out tick
		outTick, err := handler.Root().PromptAmount("out tick", outAssetID, consts.MaxUint64, nil)
		if err != nil {
			return err
		}

		// Select supply
		supply, err := handler.Root().PromptAmount(
			"supply (must be multiple of out tick)",
			outAssetID,
			balance,
			func(input uint64) error {
				if input%outTick != 0 {
					return ErrNotMultiple
				}
				return nil
			},
		)
		if err != nil {
			return err
		}

		// Confirm action
		cont, err := handler.Root().PromptContinue()
		if !cont || err != nil {
			return err
		}

		// Generate transaction
		_, _, err = sendAndWait(ctx, nil, &actions.CreateOrder{
			In:      inAssetID,
			InTick:  inTick,
			Out:     outAssetID,
			OutTick: outTick,
			Supply:  supply,
		}, cli, tcli, factory, true)
		return err
	},
}

var fillOrderCmd = &cobra.Command{
	Use: "fill-order",
	RunE: func(*cobra.Command, []string) error {
		ctx := context.Background()
		_, priv, factory, cli, tcli, err := handler.DefaultActor()
		if err != nil {
			return err
		}

		// Select inbound token
		inAssetID, err := handler.Root().PromptAsset("in assetID", true)
		if err != nil {
			return err
		}
		balance, _, err := handler.GetAssetInfo(ctx, tcli, priv.PublicKey(), inAssetID, true)
		if balance == 0 || err != nil {
			return err
		}

		// Select outbound token
		outAssetID, err := handler.Root().PromptAsset("out assetID", true)
		if err != nil {
			return err
		}
		if _, _, err := handler.GetAssetInfo(ctx, tcli, priv.PublicKey(), outAssetID, false); err != nil {
			return err
		}

		// View orders
		orders, err := tcli.Orders(ctx, actions.PairID(inAssetID, outAssetID))
		if err != nil {
			return err
		}
		if len(orders) == 0 {
			hutils.Outf("{{red}}no available orders{{/}}\n")
			hutils.Outf("{{red}}exiting...{{/}}\n")
			return nil
		}
		hutils.Outf("{{cyan}}available orders:{{/}} %d\n", len(orders))
		max := 20
		if len(orders) < max {
			max = len(orders)
		}
		for i := 0; i < max; i++ {
			order := orders[i]
			hutils.Outf(
				"%d) {{cyan}}Rate(in/out):{{/}} %.4f {{cyan}}InTick:{{/}} %s %s {{cyan}}OutTick:{{/}} %s %s {{cyan}}Remaining:{{/}} %s %s\n", //nolint:lll
				i,
				float64(order.InTick)/float64(order.OutTick),
				handler.Root().ValueString(inAssetID, order.InTick),
				handler.Root().AssetString(inAssetID),
				handler.Root().ValueString(outAssetID, order.OutTick),
				handler.Root().AssetString(outAssetID),
				handler.Root().ValueString(outAssetID, order.Remaining),
				handler.Root().AssetString(outAssetID),
			)
		}

		// Select order
		orderIndex, err := handler.Root().PromptChoice("select order", max)
		if err != nil {
			return err
		}
		order := orders[orderIndex]

		// Select input to trade
		value, err := handler.Root().PromptAmount(
			"value (must be multiple of in tick)",
			inAssetID,
			balance,
			func(input uint64) error {
				if input%order.InTick != 0 {
					return ErrNotMultiple
				}
				multiples := input / order.InTick
				requiredRemainder := order.OutTick * multiples
				if requiredRemainder > order.Remaining {
					return ErrInsufficientSupply
				}
				return nil
			},
		)
		if err != nil {
			return err
		}
		multiples := value / order.InTick
		outAmount := multiples * order.OutTick
		hutils.Outf(
			"{{orange}}in:{{/}} %s %s {{orange}}out:{{/}} %s %s\n",
			handler.Root().ValueString(inAssetID, value),
			handler.Root().AssetString(inAssetID),
			handler.Root().ValueString(outAssetID, outAmount),
			handler.Root().AssetString(outAssetID),
		)

		// Confirm action
		cont, err := handler.Root().PromptContinue()
		if !cont || err != nil {
			return err
		}

		owner, err := utils.ParseAddress(order.Owner)
		if err != nil {
			return err
		}
		_, _, err = sendAndWait(ctx, nil, &actions.FillOrder{
			Order: order.ID,
			Owner: owner,
			In:    inAssetID,
			Out:   outAssetID,
			Value: value,
		}, cli, tcli, factory, true)
		return err
	},
}

func performImport(
	ctx context.Context,
	scli *rpc.JSONRPCClient,
	dcli *rpc.JSONRPCClient,
	dtcli *trpc.JSONRPCClient,
	exportTxID ids.ID,
	priv ed25519.PrivateKey,
	factory chain.AuthFactory,
) error {
	// Select TxID (if not provided)
	var err error
	if exportTxID == ids.Empty {
		exportTxID, err = handler.Root().PromptID("export txID")
		if err != nil {
			return err
		}
	}

	// Generate warp signature (as long as >= 80% stake)
	var (
		msg                     *warp.Message
		subnetWeight, sigWeight uint64
	)
	for ctx.Err() == nil {
		msg, subnetWeight, sigWeight, err = scli.GenerateAggregateWarpSignature(ctx, exportTxID)
		if sigWeight >= (subnetWeight*4)/5 && err == nil {
			break
		}
		if err == nil {
			hutils.Outf(
				"{{yellow}}waiting for signature weight:{{/}} %d {{yellow}}observed:{{/}} %d\n",
				subnetWeight,
				sigWeight,
			)
		} else {
			hutils.Outf("{{red}}encountered error:{{/}} %v\n", err)
		}
		cont, err := handler.Root().PromptBool("try again")
		if err != nil {
			return err
		}
		if !cont {
			hutils.Outf("{{red}}exiting...{{/}}\n")
			return nil
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	wt, err := actions.UnmarshalWarpTransfer(msg.UnsignedMessage.Payload)
	if err != nil {
		return err
	}
	outputAssetID := wt.Asset
	if !wt.Return {
		outputAssetID = actions.ImportedAssetID(wt.Asset, msg.SourceChainID)
	}
	hutils.Outf(
		"%s {{yellow}}to:{{/}} %s {{yellow}}source assetID:{{/}} %s {{yellow}}output assetID:{{/}} %s {{yellow}}value:{{/}} %s {{yellow}}reward:{{/}} %s {{yellow}}return:{{/}} %t\n",
		hutils.ToID(
			msg.UnsignedMessage.Payload,
		),
		utils.Address(wt.To),
		handler.Root().AssetString(wt.Asset),
		handler.Root().AssetString(outputAssetID),
		handler.Root().ValueString(outputAssetID, wt.Value),
		handler.Root().ValueString(outputAssetID, wt.Reward),
		wt.Return,
	)
	if wt.SwapIn > 0 {
		hutils.Outf(
			"{{yellow}}asset in:{{/}} %s {{yellow}}swap in:{{/}} %s {{yellow}}asset out:{{/}} %s {{yellow}}swap out:{{/}} %s {{yellow}}swap expiry:{{/}} %d\n",
			handler.Root().AssetString(outputAssetID),
			handler.Root().ValueString(
				outputAssetID,
				wt.SwapIn,
			),
			handler.Root().AssetString(wt.AssetOut),
			handler.Root().ValueString(wt.AssetOut, wt.SwapOut),
			wt.SwapExpiry,
		)
	}
	hutils.Outf(
		"{{yellow}}signature weight:{{/}} %d {{yellow}}total weight:{{/}} %d\n",
		sigWeight,
		subnetWeight,
	)

	// Select fill
	var fill bool
	if wt.SwapIn > 0 {
		fill, err = handler.Root().PromptBool("fill")
		if err != nil {
			return err
		}
	}
	if !fill && wt.SwapExpiry > time.Now().UnixMilli() {
		return ErrMustFill
	}

	// Attempt to send dummy transaction if needed
	if err := handler.Root().SubmitDummy(ctx, dcli, func(ictx context.Context, count uint64) error {
		_, _, err = sendAndWait(ictx, nil, &actions.Transfer{
			To:    priv.PublicKey(),
			Value: count, // prevent duplicate txs
		}, dcli, dtcli, factory, false)
		return err
	}); err != nil {
		return err
	}

	// Generate transaction
	_, _, err = sendAndWait(ctx, msg, &actions.ImportAsset{
		Fill: fill,
	}, dcli, dtcli, factory, true)
	return err
}

var importAssetCmd = &cobra.Command{
	Use: "import-asset",
	RunE: func(*cobra.Command, []string) error {
		ctx := context.Background()
		currentChainID, priv, factory, dcli, dtcli, err := handler.DefaultActor()
		if err != nil {
			return err
		}

		// Select source
		_, uris, err := handler.Root().PromptChain("sourceChainID", set.Of(currentChainID))
		if err != nil {
			return err
		}
		scli := rpc.NewJSONRPCClient(uris[0])

		// Perform import
		return performImport(ctx, scli, dcli, dtcli, ids.Empty, priv, factory)
	},
}

var exportAssetCmd = &cobra.Command{
	Use: "export-asset",
	RunE: func(*cobra.Command, []string) error {
		ctx := context.Background()
		currentChainID, priv, factory, cli, tcli, err := handler.DefaultActor()
		if err != nil {
			return err
		}

		// Select token to send
		assetID, err := handler.Root().PromptAsset("assetID", true)
		if err != nil {
			return err
		}
		balance, sourceChainID, err := handler.GetAssetInfo(ctx, tcli, priv.PublicKey(), assetID, true)
		if balance == 0 || err != nil {
			return err
		}

		// Select recipient
		recipient, err := handler.Root().PromptAddress("recipient")
		if err != nil {
			return err
		}

		// Select amount
		amount, err := handler.Root().PromptAmount("amount", assetID, balance, nil)
		if err != nil {
			return err
		}

		// Determine return
		var ret bool
		if sourceChainID != ids.Empty {
			ret = true
		}

		// Select reward
		reward, err := handler.Root().PromptAmount("reward", assetID, balance-amount, nil)
		if err != nil {
			return err
		}

		// Determine destination
		destination := sourceChainID
		if !ret {
			destination, _, err = handler.Root().PromptChain("destination", set.Of(currentChainID))
			if err != nil {
				return err
			}
		}

		// Determine if swap in
		swap, err := handler.Root().PromptBool("swap on import")
		if err != nil {
			return err
		}
		var (
			swapIn     uint64
			assetOut   ids.ID
			swapOut    uint64
			swapExpiry int64
		)
		if swap {
			swapIn, err = handler.Root().PromptAmount("swap in", assetID, amount, nil)
			if err != nil {
				return err
			}
			assetOut, err = handler.Root().PromptAsset("asset out (on destination)", true)
			if err != nil {
				return err
			}
			swapOut, err = handler.Root().PromptAmount(
				"swap out (on destination)",
				assetOut,
				consts.MaxUint64,
				nil,
			)
			if err != nil {
				return err
			}
			swapExpiry, err = handler.Root().PromptTime("swap expiry")
			if err != nil {
				return err
			}
		}

		// Confirm action
		cont, err := handler.Root().PromptContinue()
		if !cont || err != nil {
			return err
		}

		// Attempt to send dummy transaction if needed
		if err := handler.Root().SubmitDummy(ctx, cli, func(ictx context.Context, count uint64) error {
			_, _, err = sendAndWait(ictx, nil, &actions.Transfer{
				To:    priv.PublicKey(),
				Value: count, // prevent duplicate txs
			}, cli, tcli, factory, false)
			return err
		}); err != nil {
			return err
		}

		// Generate transaction
		success, txID, err := sendAndWait(ctx, nil, &actions.ExportAsset{
			To:          recipient,
			Asset:       assetID,
			Value:       amount,
			Return:      ret,
			Reward:      reward,
			SwapIn:      swapIn,
			AssetOut:    assetOut,
			SwapOut:     swapOut,
			SwapExpiry:  swapExpiry,
			Destination: destination,
		}, cli, tcli, factory, true)
		if err != nil {
			return err
		}
		if !success {
			return errors.New("not successful")
		}

		// Perform import
		imp, err := handler.Root().PromptBool("perform import on destination")
		if err != nil {
			return err
		}
		if imp {
			uris, err := handler.Root().GetChain(destination)
			if err != nil {
				return err
			}
			networkID, _, _, err := cli.Network(ctx)
			if err != nil {
				return err
			}
			if err := performImport(ctx, cli, rpc.NewJSONRPCClient(uris[0]), trpc.NewJSONRPCClient(uris[0], networkID, destination), txID, priv, factory); err != nil {
				return err
			}
		}

		// Ask if user would like to switch to destination chain
		sw, err := handler.Root().PromptBool("switch default chain to destination")
		if err != nil {
			return err
		}
		if !sw {
			return nil
		}
		return handler.Root().StoreDefaultChain(destination)
	},
}
