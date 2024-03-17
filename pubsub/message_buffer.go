// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pubsub

import (
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/timer"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"go.uber.org/zap"
)

type MessageBuffer struct {
	Queue chan []byte

	l            sync.Mutex
	log          logging.Logger
	pending      [][]byte
	pendingSize  int
	targetSize   int
	maxSize      int
	maxPackSize  int
	timeout      time.Duration
	pendingTimer *timer.Timer
	closed       bool
}

func NewMessageBuffer(log logging.Logger, pending int, targetSize int, maxSize int, timeout time.Duration) *MessageBuffer {
	m := &MessageBuffer{
		Queue: make(chan []byte, pending),

		log:         log,
		pending:     [][]byte{},
		targetSize:  targetSize,
		maxSize:     maxSize,
		maxPackSize: maxSize - consts.IntLen, // account for message count in batch
		timeout:     timeout,
	}
	m.pendingTimer = timer.NewTimer(func() {
		m.l.Lock()
		defer m.l.Unlock()

		if m.closed {
			log.Debug("unable to clear pending messages", zap.Error(ErrClosed))
			return
		}
		l := len(m.pending)
		if l == 0 {
			return
		}
		if err := m.clearPending(); err != nil {
			log.Debug("unable to clear pending messages", zap.Error(err))
		} else {
			log.Debug("sent messages", zap.Int("count", l))
		}
	})
	go m.pendingTimer.Dispatch()
	return m
}

func (m *MessageBuffer) Close() error {
	m.l.Lock()
	defer m.l.Unlock()

	if m.closed {
		return ErrClosed
	}

	// Flush anything left
	//
	// It is up to the caller to ensure all of these items actually are written
	// to the connection before it is closed.
	if err := m.clearPending(); err != nil {
		m.log.Debug("unable to clear pending messages", zap.Error(err))
		return err
	}

	m.pendingTimer.Stop()
	m.closed = true
	close(m.Queue)
	return nil
}

func (m *MessageBuffer) clearPending() error {
	bm, err := CreateBatchMessage(m.maxSize, m.pending)
	if err != nil {
		// If this is failing in a loop, we will stay stuck here forever....
		panic(err) // TODO: remove this panic
	}
	select {
	case m.Queue <- bm:
	default:
		m.log.Debug("dropped pending message")
		panic("dropped message from buffer")
	}

	m.pendingSize = 0
	m.pending = [][]byte{}
	return nil
}

func (m *MessageBuffer) Send(msg []byte) error {
	m.l.Lock()
	defer m.l.Unlock()

	if m.closed {
		return ErrClosed
	}

	l := codec.BytesLen(msg)
	if l > m.maxPackSize {
		return ErrMessageTooLarge
	}

	// Clear existing buffer if too large
	if m.pendingSize+l > m.maxPackSize {
		m.pendingTimer.Cancel()
		if err := m.clearPending(); err != nil {
			return err
		}
	}

	// Add pending
	m.pendingSize += l
	m.pending = append(m.pending, msg)

	// Clear existing buffer if greater than target
	if m.pendingSize > m.targetSize {
		m.pendingTimer.Cancel()
		if err := m.clearPending(); err != nil {
			return err
		}
		return nil
	}

	// Set timer if this is the only message
	if len(m.pending) == 1 {
		m.pendingTimer.SetTimeoutIn(m.timeout)
	}
	return nil
}

func CreateBatchMessage(maxSize int, msgs [][]byte) ([]byte, error) {
	size := consts.IntLen
	for _, msg := range msgs {
		size += codec.BytesLen(msg)
	}
	msgBatch := codec.NewWriter(size, maxSize)
	msgBatch.PackInt(len(msgs))
	for _, msg := range msgs {
		msgBatch.PackBytes(msg)
	}
	return msgBatch.Bytes(), msgBatch.Err()
}

func ParseBatchMessage(maxSize int, msg []byte) ([][]byte, error) {
	msgBatch := codec.NewReader(msg, maxSize)
	msgLen := msgBatch.UnpackInt(true)
	msgs := [][]byte{}
	for i := 0; i < msgLen; i++ {
		var nextMsg []byte
		msgBatch.UnpackBytes(-1, true, &nextMsg)
		if err := msgBatch.Err(); err != nil {
			return nil, err
		}
		msgs = append(msgs, nextMsg)
	}
	return msgs, msgBatch.Err()
}
