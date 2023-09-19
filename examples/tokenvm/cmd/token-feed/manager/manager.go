// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package manager

import (
	"context"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/tokenvm/actions"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/cmd/token-feed/config"
	"github.com/ava-labs/hypersdk/examples/tokenvm/consts"
	trpc "github.com/ava-labs/hypersdk/examples/tokenvm/rpc"
	tutils "github.com/ava-labs/hypersdk/examples/tokenvm/utils"
	"github.com/ava-labs/hypersdk/pubsub"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/utils"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
)

type FeedObject struct {
	Address   string `json:"address"`
	TxID      ids.ID `json:"txID"`
	Timestamp int64  `json:"timestamp"`
	Fee       uint64 `json:"fee"`
	Memo      []byte `json:"memo"`
}

type Manager struct {
	log    logging.Logger
	config *config.Config

	tcli *trpc.JSONRPCClient

	l              sync.RWMutex
	epochStart     int64
	epochSolutions int
	feeAmount      uint64

	f sync.RWMutex
	// TODO: persist this
	feed []*FeedObject
}

func New(logger logging.Logger, config *config.Config) (*Manager, error) {
	ctx := context.TODO()
	cli := rpc.NewJSONRPCClient(config.TokenRPC)
	networkID, _, chainID, err := cli.Network(ctx)
	if err != nil {
		return nil, err
	}
	tcli := trpc.NewJSONRPCClient(config.TokenRPC, networkID, chainID)
	m := &Manager{log: logger, config: config, tcli: tcli, feed: []*FeedObject{}}
	m.epochStart = time.Now().Unix()
	m.feeAmount = m.config.MinFee
	m.log.Info("feed initialized",
		zap.String("address", m.config.Recipient),
		zap.String("fee", utils.FormatBalance(m.feeAmount, consts.Decimals)),
	)
	return m, nil
}

func (m *Manager) Run(ctx context.Context) error {
	parser, err := m.tcli.Parser(ctx)
	if err != nil {
		return err
	}
	recipientPubKey, err := m.config.RecipientPublicKey()
	if err != nil {
		return err
	}
	for ctx.Err() == nil { // handle WS client failure
		scli, err := rpc.NewWebSocketClient(m.config.TokenRPC, rpc.DefaultHandshakeTimeout, pubsub.MaxPendingMessages, pubsub.MaxReadMessageSize)
		if err != nil {
			m.log.Warn("unable to connect to RPC", zap.String("uri", m.config.TokenRPC), zap.Error(err))
			time.Sleep(10 * time.Second)
			continue
		}
		if err := scli.RegisterBlocks(); err != nil {
			m.log.Warn("unable to connect to register for blocks", zap.String("uri", m.config.TokenRPC), zap.Error(err))
			time.Sleep(10 * time.Second)
			continue
		}
		for ctx.Err() == nil {
			// Listen for blocks
			blk, results, _, err := scli.ListenBlock(ctx, parser)
			if err != nil {
				m.log.Warn("unable to listen for blocks", zap.Error(err))
				break
			}

			// Look for transactions to recipient
			for i, tx := range blk.Txs {
				action, ok := tx.Action.(*actions.Transfer)
				if !ok {
					continue
				}
				if action.To != recipientPubKey {
					continue
				}
				if len(action.Memo) == 0 {
					continue
				}
				result := results[i]
				from := auth.GetActor(tx.Auth)
				if !result.Success {
					m.log.Info("incoming message failed on-chain", zap.String("from", tutils.Address(from)), zap.String("memo", string(action.Memo)), zap.Uint64("payment", action.Value), zap.Uint64("required", m.feeAmount))
					continue
				}
				if action.Value < m.feeAmount {
					m.log.Info("incoming message did not pay enough", zap.String("from", tutils.Address(from)), zap.String("memo", string(action.Memo)), zap.Uint64("payment", action.Value), zap.Uint64("required", m.feeAmount))
					continue
				}
				addr := tutils.Address(from)
				m.l.Lock()
				m.f.Lock()
				m.feed = append([]*FeedObject{{
					Address:   addr,
					TxID:      tx.ID(),
					Timestamp: blk.Tmstmp,
					Fee:       action.Value,
					Memo:      action.Memo,
				}}, m.feed...)
				if len(m.feed) > m.config.FeedSize {
					// TODO: do this more efficiently using a rolling window
					m.feed[m.config.FeedSize] = nil // prevent memory leak
					m.feed = m.feed[:m.config.FeedSize]
				}
				m.epochSolutions++
				if m.epochSolutions >= m.config.MessagesPerEpoch {
					now := time.Now().Unix()
					elapsed := now - m.epochStart
					if elapsed < m.config.TargetDurationPerEpoch {
						m.feeAmount += m.config.FeeDelta
						m.log.Info("increasing message fee", zap.Uint64("fee", m.feeAmount))
					} else if m.feeAmount > m.config.MinFee {
						m.feeAmount += m.config.FeeDelta
						m.log.Info("decreasing message fee", zap.Uint64("fee", m.feeAmount))
					}
					m.epochSolutions = 0
					m.epochStart = now
				}
				m.log.Info("received incoming message", zap.String("from", tutils.Address(from)), zap.String("memo", string(action.Memo)), zap.Uint64("payment", action.Value), zap.Uint64("new required", m.feeAmount))
				m.f.Unlock()
				m.l.Unlock()
			}
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Sleep before trying again
		time.Sleep(10 * time.Second)
	}
	return ctx.Err()
}

func (m *Manager) GetFeedInfo(_ context.Context) (ed25519.PublicKey, uint64, error) {
	m.l.RLock()
	defer m.l.RUnlock()

	pk, err := m.config.RecipientPublicKey()
	return pk, m.feeAmount, err
}

// TODO: allow for multiple feeds
func (m *Manager) GetFeed(ctx context.Context) ([]*FeedObject, error) {
	m.f.RLock()
	defer m.f.RUnlock()

	return slices.Clone(m.feed), nil
}
