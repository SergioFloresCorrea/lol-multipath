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

type result struct {
	conn     *UdpConnection
	pingConn *UdpConnection
	ping     int64
}

type ConnectionPort struct {
	UDPConns  []*UdpConnection
	PingConns []*UdpConnection
}

type WrappedUDPPacket struct {
	ID   uuid.UUID
	Data []byte
}

type ProxyConfig struct {
	RemoteIP   net.IP
	RemotePort string
	ClientIP   string
	ClientPort string
}

type SeenHashTracker struct {
	mu       sync.Mutex
	SeenHash map[uint64]time.Time
}

// Cleans entries in the hash tracker older than `cleanupInterval`
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

// Checks if a hash is was already saved in the hash tracker.
// If it is, returns true. If it isn't, saves the timestamp with the hash
// as a key and returns false.
func (tracker *SeenHashTracker) isHashDuplicate(hash uint64) bool {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	if _, exists := tracker.SeenHash[hash]; exists {
		return true
	}
	tracker.SeenHash[hash] = time.Now()
	return false
}

// Checks if every connection to a proxy has a corresponding connection
// where to ping
func (c *ConnectionPort) CheckLengths() bool {
	return len(c.UDPConns) == len(c.PingConns)
}
