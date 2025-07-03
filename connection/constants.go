package connection

import "time"

const (
	leagueProcessName = "League of Legends"
	timeout           = 2 * time.Second
)

type UDPResult struct {
	LocalIP   string
	LocalPort int
}

type ConnectionUDP struct {
	LocalAddress string `json:"LocalAddress"`
	LocalPort    int    `json:"LocalPort"`
}
