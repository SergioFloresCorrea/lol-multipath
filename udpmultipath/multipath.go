package udpmultipath

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

const (
	updateInterval = 30 * time.Second
)

// MultipathProxy spins up a single send loop and, if dynamic==true,
// also a background ticker that updates the set of best connections.
func MultipathProxy(localIPs []net.IP, proxyAddrs, proxyPingAddrs []string, packetChan <-chan []byte, dynamic bool) error {
	// 1) Initial setup & first selection
	connSet, err := multipathSetup(localIPs, proxyAddrs, proxyPingAddrs)
	if err != nil {
		return err
	}
	if !connSet.CheckLengths() {
		return fmt.Errorf("a proxy has no corresponding ping port or listen port")
	}

	firstTime := true
	selected, err := selectBestConnections(connSet.UDPConns, connSet.PingConns, &firstTime)
	if err != nil {
		return err
	}
	if len(selected) > maxConnections {
		selected = selected[:maxConnections]
	}

	defer closeConnections(connSet.UDPConns)
	defer closeConn(connSet.PingConns)

	// bestConns is the slice sendMultipathData will use;
	var (
		bestConns = selected
		mu        sync.RWMutex
	)

	// 2) If dynamic reselection is turned on, start a ticker goroutine
	if dynamic {
		go func() {
			ticker := time.NewTicker(updateInterval)
			defer ticker.Stop()
			for range ticker.C {
				newSel, err := selectBestConnections(connSet.UDPConns, connSet.PingConns, &firstTime)
				if err != nil {
					log.Printf("warning: reselection error: %v", err)
					continue
				}
				if !sameConnections(bestConns, newSel) {
					mu.Lock()
					bestConns = newSel
					mu.Unlock()
					log.Printf("updated best connections: %d", len(newSel))
				}
				// else: no change, do nothing
			}
		}()
	}

	return sendMultipathData(packetChan, &bestConns, &mu)
}

func MultipathDirect(localIPs []net.IP, remoteAddress string, packetChan chan []byte, portChan chan []int) error {
	connSet, err := multipathSetup(localIPs, []string{remoteAddress}, nil)
	if err != nil {
		return err
	}

	if portChan != nil {
		portChan <- connSet.Ports
	}

	defer closeConnections(connSet.UDPConns)
	var bestMu sync.RWMutex

	return sendMultipathData(packetChan, &connSet.UDPConns, &bestMu)
}

func multipathSetup(localIPs []net.IP, targetsAddr, targetPingAddr []string) (ConnectionPort, error) {
	dialers, err := createDialers(localIPs)
	if err != nil {
		return ConnectionPort{}, err
	}

	localToTargetsConn, err := createConnections(dialers, targetsAddr, targetPingAddr)
	if err != nil {
		return ConnectionPort{}, err
	}
	return localToTargetsConn, nil
}

// sendMultipathData reads from packetChan until closed,
// and fan-outs each packet to the current bestConns slice.
// It uses each UdpConnection’s own mu to serialize .Write calls.
func sendMultipathData(packetChan <-chan []byte, selConnsPtr *[]*UdpConnection, bestMu *sync.RWMutex) error {
	downSince := make(map[*UdpConnection]time.Time)
	done := make(chan struct{})
	var downSinceMu sync.RWMutex
	var wgProbe sync.WaitGroup
	wgProbe.Add(1)

	// Attempts a "probe" write to see if it's back up every 10 seconds.
	go func() {
		defer wgProbe.Done()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				downSinceMu.RLock()
				toProbe := make([]*UdpConnection, 0, len(downSince))
				for uc, t0 := range downSince {
					if time.Since(t0) >= 10*time.Second {
						toProbe = append(toProbe, uc)
					}
				}
				downSinceMu.RUnlock()
				var wgProbe sync.WaitGroup
				for _, uc := range toProbe {
					wgProbe.Add(1)
					go func(udpConn *UdpConnection) {
						defer wgProbe.Done()
						udpConn.mu.Lock()
						defer udpConn.mu.Unlock()

						if _, err := udpConn.conn.Write([]byte{0}); err == nil {
							log.Printf("Recovered connection %s->%s (after probe)",
								udpConn.conn.LocalAddr(), udpConn.conn.RemoteAddr())
							downSinceMu.Lock()
							delete(downSince, udpConn)
							downSinceMu.Unlock()
						}
					}(uc)
				}
				wgProbe.Wait()
			}
		}
	}()

	for pkt := range packetChan {
		// grab a snapshot of the current bestConns
		bestMu.RLock()
		selconns := append([]*UdpConnection{}, *selConnsPtr...)
		conns := make([]*UdpConnection, 0, maxConnections)
		for _, uc := range selconns {
			downSinceMu.RLock()
			_, down := downSince[uc]
			downSinceMu.RUnlock()
			if !down {
				conns = append(conns, uc)
				if len(conns) == maxConnections {
					break
				}
			}
		}

		bestMu.RUnlock()

		var wg sync.WaitGroup
		for _, uc := range conns {
			wg.Add(1)
			go func(udpConn *UdpConnection, packet []byte) {
				defer wg.Done()
				udpConn.mu.Lock()
				defer udpConn.mu.Unlock()

				if _, err := udpConn.conn.Write(packet); err != nil {
					downSinceMu.RLock()
					_, down := downSince[udpConn]
					downSinceMu.RUnlock()
					if !down { // first time we see it is not down
						log.Printf("Error writing to %v: %v, connection is down; excluding until probe recovers", udpConn.conn.RemoteAddr(), err)
						downSinceMu.Lock()
						downSince[udpConn] = time.Now()
						downSinceMu.Unlock()
					}
					return
				}
			}(uc, pkt)
		}
		wg.Wait()
	}
	close(done)
	wgProbe.Wait()
	return nil
}

func createDialers(localIPs []net.IP) ([]net.Dialer, error) {
	dialers := make([]net.Dialer, 0)

	for _, localIP := range localIPs {
		localAddr := &net.UDPAddr{IP: localIP}
		dialer := net.Dialer{
			LocalAddr: localAddr,
		}
		dialers = append(dialers, dialer)
	}

	return dialers, nil
}

func createConnections(dialers []net.Dialer, targetsAddr, targetsPingAddr []string) (ConnectionPort, error) {
	localToTargetsConn := ConnectionPort{}

	var conn net.Conn
	var lport int
	var connPing net.Conn
	var err error

	for _, localDialer := range dialers {
		for _, addr := range targetsAddr {
			conn, err = localDialer.Dial("udp", addr)
			if err != nil {
				closeConnections(localToTargetsConn.UDPConns)
				return ConnectionPort{}, err
			}
			lport = conn.LocalAddr().(*net.UDPAddr).Port
		}
		for _, addr := range targetsPingAddr {
			connPing, err = localDialer.Dial("udp", addr)
			if err != nil {
				closeConnections(localToTargetsConn.UDPConns)
				return ConnectionPort{}, err
			}
		}
		localToTargetsConn.UDPConns = append(localToTargetsConn.UDPConns, &UdpConnection{mu: sync.Mutex{}, conn: conn})
		localToTargetsConn.Ports = append(localToTargetsConn.Ports, lport)
		localToTargetsConn.PingConns = append(localToTargetsConn.PingConns, connPing)
	}

	return localToTargetsConn, nil
}

func closeConnections(connections []*UdpConnection) {
	for i := range connections {
		connections[i].conn.Close()
	}
}

func closeConn(connections []net.Conn) {
	for i := range connections {
		connections[i].Close()
	}
}

// sameConnections returns true iff the two slices contain
// the same set of pointers in the same order.
func sameConnections(a, b []*UdpConnection) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
