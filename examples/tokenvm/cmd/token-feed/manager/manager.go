// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package manager

import (
	"context"
	"encoding/json"
	"slices"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/timer"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/tokenvm/actions"
	"github.com/ava-labs/hypersdk/examples/tokenvm/cmd/token-feed/config"
	"github.com/ava-labs/hypersdk/examples/tokenvm/consts"
	trpc "github.com/ava-labs/hypersdk/examples/tokenvm/rpc"
	"github.com/ava-labs/hypersdk/pubsub"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/utils"
	"go.uber.org/zap"
)

type FeedContent struct {
	Message string `json:"message"`
	URL     string `json:"url"`
}

type FeedObject struct {
	Address   string `json:"address"`
	TxID      ids.ID `json:"txID"`
	Timestamp int64  `json:"timestamp"`
	Fee       uint64 `json:"fee"`

	Content *FeedContent `json:"content"`
}

type Manager struct {
	log    logging.Logger
	config *config.Config

	tcli *trpc.JSONRPCClient

	l             sync.RWMutex
	t             *timer.Timer
	epochStart    int64
	epochMessages int
	feeAmount     uint64

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
	m.t = timer.NewTimer(m.updateFee)
	return m, nil
}

func (m *Manager) updateFee() {
	m.l.Lock()
	defer m.l.Unlock()

	// If time since [epochStart] is within half of the target duration,
	// we attempted to update fee when we just reset during block processing.
	now := time.Now().Unix()
	if now-m.epochStart < m.config.TargetDurationPerEpoch/2 {
		return
	}

	// Decrease fee if there are no messages in this epoch
	if m.feeAmount > m.config.MinFee && m.epochMessages == 0 {
		m.feeAmount -= m.config.FeeDelta
		m.log.Info("decreasing message fee", zap.Uint64("fee", m.feeAmount))
	}
	m.epochMessages = 0
	m.epochStart = time.Now().Unix()
	m.t.SetTimeoutIn(time.Duration(m.config.TargetDurationPerEpoch) * time.Second)
}

func (m *Manager) Run(ctx context.Context) error {
	// Start update timer
	m.t.SetTimeoutIn(time.Duration(m.config.TargetDurationPerEpoch) * time.Second)
	go m.t.Dispatch()
	defer m.t.Stop()

	parser, err := m.tcli.Parser(ctx)
	if err != nil {
		return err
	}
	recipientAddr, err := m.config.RecipientAddress()
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
				if action.To != recipientAddr {
					continue
				}
				if len(action.Memo) == 0 {
					continue
				}
				result := results[i]
				from := tx.Auth.Actor()
				fromStr := codec.MustAddressBech32(consts.HRP, from)
				if !result.Success {
					m.log.Info("incoming message failed on-chain", zap.String("from", fromStr), zap.String("memo", string(action.Memo)), zap.Uint64("payment", action.Value), zap.Uint64("required", m.feeAmount))
					continue
				}
				if action.Value < m.feeAmount {
					m.log.Info("incoming message did not pay enough", zap.String("from", fromStr), zap.String("memo", string(action.Memo)), zap.Uint64("payment", action.Value), zap.Uint64("required", m.feeAmount))
					continue
				}

				var c FeedContent
				if err := json.Unmarshal(action.Memo, &c); err != nil {
					m.log.Info("incoming message could not be parsed", zap.String("from", fromStr), zap.String("memo", string(action.Memo)), zap.Uint64("payment", action.Value), zap.Error(err))
					continue
				}
				if len(c.Message) == 0 {
					m.log.Info("incoming message was empty", zap.String("from", fromStr), zap.String("memo", string(action.Memo)), zap.Uint64("payment", action.Value))
					continue
				}
				// TODO: pre-verify URLs
				m.l.Lock()
				m.f.Lock()
				m.feed = append([]*FeedObject{{
					Address:   fromStr,
					TxID:      tx.ID(),
					Timestamp: blk.Tmstmp,
					Fee:       action.Value,
					Content:   &c,
				}}, m.feed...)
				if len(m.feed) > m.config.FeedSize {
					// TODO: do this more efficiently using a rolling window
					m.feed[m.config.FeedSize] = nil // prevent memory leak
					m.feed = m.feed[:m.config.FeedSize]
				}
				m.epochMessages++
				if m.epochMessages >= m.config.MessagesPerEpoch {
					m.feeAmount += m.config.FeeDelta
					m.log.Info("increasing message fee", zap.Uint64("fee", m.feeAmount))
					m.epochMessages = 0
					m.epochStart = time.Now().Unix()
					m.t.Cancel()
					m.t.SetTimeoutIn(time.Duration(m.config.TargetDurationPerEpoch) * time.Second)
				}
				m.log.Info("received incoming message", zap.String("from", fromStr), zap.String("memo", string(action.Memo)), zap.Uint64("payment", action.Value), zap.Uint64("new required", m.feeAmount))
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

func (m *Manager) GetFeedInfo(_ context.Context) (codec.Address, uint64, error) {
	m.l.RLock()
	defer m.l.RUnlock()

	addr, err := m.config.RecipientAddress()
	return addr, m.feeAmount, err
}

// TODO: allow for multiple feeds
func (m *Manager) GetFeed(context.Context) ([]*FeedObject, error) {
	m.f.RLock()
	defer m.f.RUnlock()

	return slices.Clone(m.feed), nil
}
