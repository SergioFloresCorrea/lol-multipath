package connection

import (
	"log"
	"time"
)

type TCPResult struct {
	RemoteIPs []string
	LocalIPs  []string
	Err       error
}

type UDPResult struct {
	LocalIP   string
	LocalPort string
	Err       error
}

func WaitForLeagueAndResolve(protocol string, interval time.Duration, done chan struct{},
	tcpResultChan chan TCPResult, udpResultChan chan UDPResult,
) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			log.Println("League watcher stopped.")
			return

		case <-ticker.C:
			switch protocol {
			case "TCP":
				remoteIPs, localIPs, err := ResolveRiotIP("/tmp/netstat.txt", protocol)
				if err == nil {
					tcpResultChan <- TCPResult{RemoteIPs: remoteIPs, LocalIPs: localIPs}
					close(done)
					return
				}
				log.Printf("TCP detection error: %v\n", err)

			case "UDP":
				connection, err := GetUDPConnection()
				if err == nil {
					udpResultChan <- UDPResult{LocalIP: connection.LocalAddress, LocalPort: connection.LocalPort}
					close(done)
					return
				}
				// log.Printf("UDP detection error: %v\n", err)
			}
		}
	}
}
