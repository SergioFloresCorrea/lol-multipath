package udpmultipath

import (
	"net"
	"sync"
)

type UdpConnection struct {
	mu   sync.Mutex
	conn net.Conn
}
