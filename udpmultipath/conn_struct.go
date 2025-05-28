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

type SeenIDTracker struct {
	mu      sync.Mutex
	SeenIDs map[uuid.UUID]time.Time
}
