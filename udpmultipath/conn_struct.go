package udpmultipath

import (
	"net"
	"sync"
	"time"
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

type ProxyConfig struct {
	RemoteIP   net.IP
	RemotePort string
	ClientIP   string
	ClientPort int
}

type SeenHashTracker struct {
	mu              sync.Mutex
	SeenHash        map[uint64]time.Time
	cleanupInterval time.Duration
}

// Creates blank new tracker
func (cfg *Config) newTracker() *SeenHashTracker {
	return &SeenHashTracker{SeenHash: make(map[uint64]time.Time), cleanupInterval: cfg.CleanupInterval}
}

// Cleans entries in the hash tracker older than `cleanupInterval`
func (tracker *SeenHashTracker) cleanupHash() {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	now := time.Now()
	for hash, ts := range tracker.SeenHash {
		if now.Sub(ts) >= tracker.cleanupInterval {
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
