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
	pingConn net.Conn
	ping     int64
}

type ConnectionPort struct {
	UDPConns  []*UdpConnection
	Ports     []int
	PingConns []net.Conn
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

type connectionStats struct {
	proxyAddr string
	lossRate  float64
	prevRTT   time.Duration
	latency   time.Duration
	jitter    time.Duration
	sent      int
	recv      int
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

func (c *ConnectionPort) CheckLengths() bool {
	/*
		Only has meaning if we are using proxies as only they listen to pings.
	*/
	check1 := len(c.UDPConns) == len(c.Ports)
	check2 := len(c.UDPConns) == len(c.PingConns)
	check3 := len(c.Ports) == len(c.PingConns)
	return check1 && check2 && check3
}
