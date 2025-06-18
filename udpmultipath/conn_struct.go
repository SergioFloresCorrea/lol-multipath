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
