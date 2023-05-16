package pubsub

import (
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/timer"
	"github.com/ava-labs/hypersdk/codec"
	"go.uber.org/zap"
)

type MessageBuffer struct {
	Queue chan []byte

	l            sync.Mutex
	pending      [][]byte
	pendingSize  int
	maxSize      int
	timeout      time.Duration
	pendingTimer *timer.Timer
}

func NewMessageBuffer(log logging.Logger, pending int, maxSize int, timeout time.Duration) *MessageBuffer {
	m := &MessageBuffer{
		Queue: make(chan []byte, pending),

		pending: [][]byte{},
		maxSize: maxSize,
		timeout: timeout,
	}
	m.pendingTimer = timer.NewTimer(func() {
		m.l.Lock()
		defer m.l.Unlock()

		if len(m.pending) == 0 {
			return
		}
		if err := m.clearPending(); err != nil {
			log.Error("unable to clear pending messages", zap.Error(err))
		}
	})
	return m
}

func (m *MessageBuffer) clearPending() error {
	bm, err := CreateBatchMessage(m.maxSize, m.pending)
	if err != nil {
		return err
	}
	m.Queue <- bm
	m.pendingSize = 0
	m.pending = [][]byte{}
	return nil
}

func (m *MessageBuffer) Send(msg []byte) error {
	m.l.Lock()
	defer m.l.Unlock()

	l := len(msg)
	if l > m.maxSize {
		return ErrMessageTooLarge
	}

	// Clear existing buffer if too large
	if m.pendingSize+l > m.maxSize {
		m.pendingTimer.Cancel()
		if err := m.clearPending(); err != nil {
			return err
		}
	}

	m.pendingSize += l
	m.pending = append(m.pending, msg)
	if len(m.pending) == 1 {
		m.pendingTimer.SetTimeoutIn(m.timeout)
	}
	return nil
}

func CreateBatchMessage(maxSize int, msgs [][]byte) ([]byte, error) {
	msgBatch := codec.NewWriter(maxSize)
	msgBatch.PackInt(len(msgs))
	for _, msg := range msgs {
		msgBatch.PackBytes(msg)
	}
	return msgBatch.Bytes(), msgBatch.Err()
}

func ParseBatchMessage(maxSize int, maxLen int, msg []byte) ([][]byte, error) {
	msgBatch := codec.NewReader(msg, maxSize)
	msgLen := msgBatch.UnpackInt(true)
	if msgLen > maxLen {
		return nil, ErrTooManyMessages
	}
	msgs := make([][]byte, msgLen)
	for i := 0; i < msgLen; i++ {
		msgBatch.UnpackBytes(-1, true, &msgs[i])
	}
	return msgs, msgBatch.Err()
}
