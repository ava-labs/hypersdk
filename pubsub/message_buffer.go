// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pubsub

import (
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/timer"
	"go.uber.org/zap"
)

type MessageBuffer struct {
	Queue chan []byte

	l            sync.Mutex
	log          logging.Logger
	pending      [][]byte
	pendingSize  int
	maxSize      int
	timeout      time.Duration
	pendingTimer *timer.Timer
	closed       bool
}

func NewMessageBuffer(log logging.Logger, pending int, maxSize int, timeout time.Duration) *MessageBuffer {
	m := &MessageBuffer{
		Queue: make(chan []byte, pending),

		log:     log,
		pending: [][]byte{},
		maxSize: maxSize,
		timeout: timeout,
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
		m.clearPending()
		log.Debug("sent messages", zap.Int("count", l))
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
	m.clearPending()

	m.pendingTimer.Stop()
	m.closed = true
	close(m.Queue)
	return nil
}

func (m *MessageBuffer) clearPending() {
	bm := CreateBatchMessage(m.pending)
	select {
	case m.Queue <- bm:
	default:
		m.log.Debug("dropped pending message")
	}

	m.pendingSize = 0
	m.pending = [][]byte{}
}

func (m *MessageBuffer) Send(msg []byte) error {
	m.l.Lock()
	defer m.l.Unlock()

	if m.closed {
		return ErrClosed
	}

	l := len(msg)
	if l > m.maxSize {
		return ErrMessageTooLarge
	}

	// Clear existing buffer if too large
	if m.pendingSize+l > m.maxSize {
		m.pendingTimer.Cancel()
		m.clearPending()
	}

	m.pendingSize += l
	m.pending = append(m.pending, msg)
	if len(m.pending) == 1 {
		m.pendingTimer.SetTimeoutIn(m.timeout)
	}
	return nil
}
