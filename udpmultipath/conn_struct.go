package udpmultipath

import (
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
)

type UdpConnection struct {
	mu   sync.Mutex
	conn net.Conn
}

type ConnectionPort struct {
	Conns []UdpConnection
	Ports []int
}

type WrappedUDPPacket struct {
	ID   uuid.UUID
	Data []byte
}

type IpLatency struct {
	ip      string
	latency time.Duration
}

type SeenHashTracker struct {
	mu       sync.Mutex
	SeenHash map[[32]byte]time.Time
}

func (tracker *SeenHashTracker) cleanupHash() {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	now := time.Now()
	for hash, ts := range tracker.SeenHash {
		if now.Sub(ts) > cleanupInterval {
			delete(tracker.SeenHash, hash)
		}
	}
}

func (tracker *SeenHashTracker) isHashDuplicate(hash [32]byte) bool {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	if _, exists := tracker.SeenHash[hash]; exists {
		return true
	}
	tracker.SeenHash[hash] = time.Now()
	return false
}
