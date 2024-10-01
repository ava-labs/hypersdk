package spam

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/pubsub"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/utils"
)


type txWrapper struct {
	id       ids.ID
	expiry   int64
	issuance int64
}

func (w *txWrapper) ID() ids.ID {
	return w.id
}

func (w *txWrapper) Expiry() int64 {
	return w.expiry
}

type txIssuer struct {
	c *rpc.JSONRPCClient
	d *rpc.WebSocketClient

	l         sync.Mutex
	name      string
	uri       int
	abandoned error
}

func startConfirmer(cctx context.Context, c *rpc.WebSocketClient) {
	issuerWg.Add(1)
	go func() {
		for {
			recv, rawMsgSize, txID, status, err := c.ListenTx(context.TODO())
			if err != nil {
				utils.Outf("{{red}}unable to listen for tx{{/}}: %v\n", err)
				return
			}
			received.Add(1)
			receivedBytes.Add(int64(rawMsgSize))
			tw, ok := pending.Remove(txID)
			if !ok {
				// This could happen if we've removed the transaction from pending after [pendingExpiryBuffer].
				// This will be counted by the loop that did that, so we can just continue.
				continue
			}
			l.Lock()
			switch status {
			case rpc.TxSuccess:
				successTxs++
			case rpc.TxFailed:
				failedTxs++
			case rpc.TxExpired:
				expiredTxs++
			case rpc.TxInvalid:
				invalidTxs++
			default:
				utils.Outf("{{red}}unknown tx status{{/}}: %d\n", status)
				return
			}
			if status <= rpc.TxExpired {
				confirmationTime += uint64(recv - tw.issuance)
			}
			totalTxs++
			l.Unlock()
		}
	}()
	go func() {
		defer func() {
			_ = c.Close()
			issuerWg.Done()
		}()

		<-cctx.Done()
		start := time.Now()
		for time.Since(start) < issuerShutdownTimeout {
			if c.Closed() {
				return
			}
			if pending.Len() == 0 {
				return
			}
			time.Sleep(500 * time.Millisecond)
		}
		utils.Outf("{{orange}}issuer shutdown timeout{{/}}\n")
	}()
}

func issueTransfer(cctx context.Context, issuer *txIssuer, issuerIndex int, feePerTx uint64, factory chain.AuthFactory, action chain.Action) error {
	_, tx, err := issuer.c.GenerateTransactionManual(parser, nil, action, factory, feePerTx)
	if err != nil {
		utils.Outf("{{orange}}failed to generate tx (issuer: %d):{{/}} %v\n", issuerIndex, err)
		return err
	}
	pending.Add(&txWrapper{id: tx.ID(), expiry: tx.Expiry(), issuance: time.Now().UnixMilli()}, false)
	if err := issuer.d.RegisterTx(tx); err != nil {
		issuer.l.Lock()
		if issuer.d.Closed() {
			if issuer.abandoned != nil {
				issuer.l.Unlock()
				utils.Outf("{{orange}}issuer abandoned:{{/}} %v\n", issuer.abandoned)
				return issuer.abandoned
			}
			// recreate issuer
			utils.Outf("{{orange}}re-creating issuer:{{/}} %d {{orange}}uri:{{/}} %d {{red}}err:{{/}} %v\n", issuerIndex, issuer.uri, err)
			dcli, err := rpc.NewWebSocketClient(
				uris[issuer.name],
				rpc.DefaultHandshakeTimeout,
				maxPendingMessages,
				consts.MTU,
				pubsub.MaxReadMessageSize,
			) // we write the max read
			if err != nil {
				issuer.abandoned = err
				issuer.l.Unlock()
				utils.Outf("{{orange}}could not re-create closed issuer:{{/}} %v\n", err)
				return err
			}
			issuer.d = dcli
			issuer.l.Unlock()
			utils.Outf("{{green}}re-created closed issuer:{{/}} %d (%v)\n", issuerIndex, err)
			startConfirmer(cctx, dcli)
		} else {
			// This typically happens when the issuer errors and is replaced by a new one in
			// a different goroutine.
			issuer.l.Unlock()
			utils.Outf("{{orange}}failed to register tx (issuer: %d):{{/}} %v\n", issuerIndex, err)
		}
		return nil
	}
	sent.Add(1)
	sentBytes.Add(int64(len(tx.Bytes())))
	return nil
}

func getRandomIssuer(issuers []*txIssuer) (int, *txIssuer) {
	index := rand.Int() % len(issuers)
	return index, issuers[index]
}

func uniqueBytes() []byte {
	return binary.BigEndian.AppendUint64(nil, uint64(issuedTxs.Add(1)))
}

type sendable struct {
	action chain.Action
	sender chain.AuthFactory
}

type reliableSender struct {
	sends int

	cli    *rpc.JSONRPCClient
	dcli   *rpc.WebSocketClient
	parser chain.Parser

	target     int
	maxPending int
	feePerTx   uint64

	issuance     chan *sendable
	outstandingL sync.RWMutex
	outstanding  map[ids.ID]*sendable
	done         chan struct{}
}

func NewReliableSender(sends int, cli *rpc.JSONRPCClient, dcli *rpc.WebSocketClient, parser chain.Parser, target int, maxPending int, feePerTx uint64) *reliableSender {
	r := &reliableSender{
		sends: sends,

		cli:    cli,
		dcli:   dcli,
		parser: parser,

		target:     target,
		maxPending: maxPending,
		feePerTx:   feePerTx,

		issuance:    make(chan *sendable, maxPending),
		outstanding: map[ids.ID]*sendable{},
		done:        make(chan struct{}),
	}
	go r.run()
	return r
}

func (r *reliableSender) pending() int {
	r.outstandingL.RLock()
	defer r.outstandingL.RUnlock()

	return len(r.outstanding)
}

func (r *reliableSender) addOutstanding(txID ids.ID, s *sendable) {
	r.outstandingL.Lock()
	defer r.outstandingL.Unlock()

	r.outstanding[txID] = s
}

func (r *reliableSender) removeOutstanding(txID ids.ID) *sendable {
	r.outstandingL.Lock()
	defer r.outstandingL.Unlock()

	action := r.outstanding[txID]
	delete(r.outstanding, txID)
	return action
}

func (r *reliableSender) run() {
	// Issue txs
	go func() {
		var (
			start = time.Now()
			sent  = 0
		)
		for s := range r.issuance {
			for r.pending() > r.maxPending {
				time.Sleep(100 * time.Millisecond)
			}
			_, tx, err := r.cli.GenerateTransactionManual(r.parser, nil, s.action, s.sender, r.feePerTx)
			if err != nil {
				panic(err)
			}
			r.addOutstanding(tx.ID(), s)
			if err := r.dcli.RegisterTx(tx); err != nil {
				panic(fmt.Errorf("%w: failed to register tx", err))
			}

			// Sleep to ensure we don't exceed tps
			sent++
			if sent%r.target == 0 && sent > 0 {
				sleepTime := max(0, time.Second-time.Since(start))
				time.Sleep(sleepTime)
				start = time.Now()
				sent = 0
			}
		}
	}()

	// Confirm txs
	go func() {
		var (
			start   = time.Now()
			success = 0
			errors  = 0
		)
		for success < r.sends {
			_, _, txID, status, err := r.dcli.ListenTx(ctx)
			if err != nil {
				panic(err)
			}
			action := r.removeOutstanding(txID)
			if status != rpc.TxSuccess {
				// Should never happen
				r.issuance <- action
				errors++
				continue
			}
			success++
			if success%1000 == 0 && success != 0 {
				rate := time.Since(start) / time.Duration(success)
				utils.Outf(
					"{{yellow}}distributed funds:{{/}} %d/%d (errors=%d pending=%d etr=%v)\n",
					success,
					r.sends,
					errors,
					r.pending(),
					rate*time.Duration(r.sends-success),
				)
			}
		}
		close(r.issuance)
		close(r.done)
	}()
}

func (r *reliableSender) Send(sender chain.AuthFactory, action chain.Action) {
	r.issuance <- &sendable{action: action, sender: sender}
}

func (r *reliableSender) Wait(ctx context.Context) error {
	select {
	case <-r.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
