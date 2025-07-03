package udpmultipath

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// MultipathProxy spins up a single send loop and, if dynamic==true,
// also a background ticker that updates the set of best connections.
func (cfg *Config) MultipathProxy(ctx context.Context, localIPs []net.IP, proxyAddrs, proxyPingAddrs []string, packetChan <-chan []byte) error {
	// 1) Initial setup & first selection
	connSet, err := multipathSetup(localIPs, proxyAddrs, proxyPingAddrs)
	if err != nil {
		return err
	}
	if !connSet.CheckLengths() {
		return fmt.Errorf("a proxy has no corresponding ping port or listen port")
	}

	firstTime := true
	bestConns := cfg.selectBestConnections(connSet.UDPConns, connSet.PingConns, &firstTime)
	if err != nil {
		return err
	}
	if len(bestConns) > cfg.MaxConnections {
		bestConns = bestConns[:cfg.MaxConnections]
	}

	defer closeConnections(connSet.UDPConns)
	defer closeConnections(connSet.PingConns)

	// bestConns is the slice sendMultipathData will use;
	var mu sync.RWMutex

	// 2) If dynamic reselection is turned on, start a ticker goroutine
	if cfg.Dynamic {
		go func() {
			ticker := time.NewTicker(cfg.UpdateInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					newSel := cfg.selectBestConnections(connSet.UDPConns, connSet.PingConns, &firstTime)
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
			}
		}()
	}

	return cfg.sendMultipathData(ctx, packetChan, &bestConns, &mu)
}

// Creates and returns the connections from the local IPs to the target addresses and ping addresses.
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
func (cfg *Config) sendMultipathData(ctx context.Context, packetChan <-chan []byte, selConnsPtr *[]*UdpConnection, bestMu *sync.RWMutex) error {
	downSince := make(map[*UdpConnection]time.Time)
	var downSinceMu sync.RWMutex
	var wgProbe sync.WaitGroup
	wgProbe.Add(1)

	// Attempts a "probe" write to see if it's back up every 10 seconds.
	go func() {
		defer wgProbe.Done()
		ticker := time.NewTicker(cfg.ProbeInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
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

				var wg sync.WaitGroup
				for _, uc := range toProbe {
					wg.Add(1)
					go func(udpConn *UdpConnection) {
						defer wg.Done()
						udpConn.mu.Lock()
						defer udpConn.mu.Unlock()

						deadline := time.Now().Add(min(1*time.Second, cfg.ProbeInterval))
						if err := udpConn.conn.SetWriteDeadline(deadline); err != nil {
							log.Printf("Failed to set write deadline: %v", err)
						}

						if _, err := udpConn.conn.Write([]byte{0}); err == nil {
							log.Printf("Recovered connection %s->%s (after probe)",
								udpConn.conn.LocalAddr(), udpConn.conn.RemoteAddr())
							downSinceMu.Lock()
							delete(downSince, udpConn)
							downSinceMu.Unlock()
						}
					}(uc)
				}
				wg.Wait()
			}
		}
	}()

	selconns := make([]*UdpConnection, cfg.MaxConnections) // reusable buffer
	for {
		select {
		case <-ctx.Done():
			wgProbe.Wait()
			return nil
		case pkt := <-packetChan:
			// grab a snapshot of the current bestConns
			bestMu.RLock()
			n := copy(selconns, *selConnsPtr) // copy returns #elements copied
			bestMu.RUnlock()
			conns := make([]*UdpConnection, 0, cfg.MaxConnections)
			for _, uc := range selconns[:n] {
				downSinceMu.RLock()
				_, down := downSince[uc]
				downSinceMu.RUnlock()
				if !down {
					conns = append(conns, uc)
					if len(conns) == cfg.MaxConnections {
						break
					}
				}
			}

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
	}
}

// Creates dialers for every local IP. Its behaviour is not tested for IPv6
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

// Creates connections from the dialers to the target's addresses and target's ping addresses (must use IPv4 to work).
// It returns the connections in a single struct that stores them one-to-one with the same index.
// If `len(targetsAddr) != len(targetsPingAddr)`, a further check (`CheckLengths`) will fail
// This function assumes a one-to-one correspondence between `targetsAddr` and `targetsPingAddr`.
func createConnections(dialers []net.Dialer, targetsAddr, targetsPingAddr []string) (ConnectionPort, error) {
	localToTargetsConn := ConnectionPort{}
	numAddr := len(targetsAddr)

	for _, localDialer := range dialers {
		for idx := range numAddr {
			conn, err := localDialer.Dial("udp", targetsAddr[idx])
			if err != nil {
				closeConnections(localToTargetsConn.UDPConns)
				return ConnectionPort{}, fmt.Errorf("connection to address %v couldn't be resolved: %w", targetsAddr[idx], err)
			}
			connPing, err := localDialer.Dial("udp", targetsPingAddr[idx])
			if err != nil {
				closeConnections(localToTargetsConn.UDPConns)
				closeConnections(localToTargetsConn.PingConns)
				return ConnectionPort{}, fmt.Errorf("connection to ping address %v couldn't be resolved: %w", targetsPingAddr[idx], err)
			}
			localToTargetsConn.UDPConns = append(localToTargetsConn.UDPConns, &UdpConnection{mu: sync.Mutex{}, conn: conn})
			localToTargetsConn.PingConns = append(localToTargetsConn.PingConns, &UdpConnection{mu: sync.Mutex{}, conn: connPing})
		}
	}

	return localToTargetsConn, nil
}

// Closes all UdpConnections.
func closeConnections(connections []*UdpConnection) {
	for i := range connections {
		connections[i].conn.Close()
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
