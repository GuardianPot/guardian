package devicechannel

import (
	"errors"
	"sync"

	devicev1 "github.com/GuardianPot/guardian/apps/edge-agent/internal/devicechannel/gen/guardian/device/v1"
	"google.golang.org/protobuf/proto"
)

const (
	MaxMessageBytes = 1 << 20
	MaxHeaderBytes  = 16 << 10
	MaxQueueFrames  = 64
	MaxQueueBytes   = 4 << 20
)

var ErrBackpressure = errors.New("device channel queue is saturated")

type queuedRequest struct {
	frame *devicev1.ConnectRequest
	size  int
}

type requestQueue struct {
	mu    sync.Mutex
	items []queuedRequest
	bytes int
	ready chan struct{}
}

func newRequestQueue() *requestQueue {
	return &requestQueue{items: make([]queuedRequest, 0, MaxQueueFrames), ready: make(chan struct{}, 1)}
}

func (q *requestQueue) enqueue(frame *devicev1.ConnectRequest) error {
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
	q.items = append(q.items, queuedRequest{frame: frame, size: size})
	q.bytes += size
	q.signal()
	return nil
}

func (q *requestQueue) peek() (queuedRequest, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return queuedRequest{}, false
	}
	return q.items[0], true
}

func (q *requestQueue) sent(item queuedRequest) {
	q.mu.Lock()
	if len(q.items) > 0 && q.items[0].frame == item.frame {
		q.items = q.items[1:]
		q.bytes -= item.size
		if q.bytes < 0 {
			q.bytes = 0
		}
	}
	if len(q.items) > 0 {
		q.signal()
	}
	q.mu.Unlock()
}

func (q *requestQueue) wake() {
	q.mu.Lock()
	if len(q.items) > 0 {
		q.signal()
	}
	q.mu.Unlock()
}

func (q *requestQueue) signal() {
	select {
	case q.ready <- struct{}{}:
	default:
	}
}
