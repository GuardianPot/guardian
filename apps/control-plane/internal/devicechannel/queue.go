package devicechannel

import (
	"errors"
	"sync"

	devicev1 "github.com/GuardianPot/guardian/apps/control-plane/internal/devicechannel/gen/guardian/device/v1"
	"google.golang.org/protobuf/proto"
)

const (
	MaxMessageBytes = 1 << 20
	MaxHeaderBytes  = 16 << 10
	MaxQueueFrames  = 64
	MaxQueueBytes   = 4 << 20
)

var (
	ErrBackpressure = errors.New("device channel queue is saturated")
	ErrNotConnected = errors.New("device is not connected")
)

type queuedResponse struct {
	frame *devicev1.ConnectResponse
	size  int
}

type responseQueue struct {
	items chan queuedResponse
	mu    sync.Mutex
	bytes int
}

func newResponseQueue() *responseQueue {
	return &responseQueue{items: make(chan queuedResponse, MaxQueueFrames)}
}

func (q *responseQueue) enqueue(frame *devicev1.ConnectResponse) error {
	if frame == nil {
		return errors.New("device channel frame is required")
	}
	size := proto.Size(frame)
	if size < 1 || size > MaxMessageBytes {
		return errors.New("device channel frame exceeds message limit")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) >= MaxQueueFrames || q.bytes+size > MaxQueueBytes {
		return ErrBackpressure
	}
	q.items <- queuedResponse{frame: frame, size: size}
	q.bytes += size
	return nil
}

func (q *responseQueue) sent(item queuedResponse) {
	q.mu.Lock()
	q.bytes -= item.size
	if q.bytes < 0 {
		q.bytes = 0
	}
	q.mu.Unlock()
}
