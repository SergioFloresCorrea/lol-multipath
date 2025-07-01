package connection

import "time"

const (
	leagueProcessName = "League of Legends"
	timeout           = 2 * time.Second
)

type UDPResult struct {
	LocalIP   string
	LocalPort string
}

type ConnectionUDP struct {
	LocalAddress string
	LocalPort    string
}
