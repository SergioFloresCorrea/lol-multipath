package udpmultipath

import (
	"log"
	"math/rand"
	"net"
	"sync"
)

const maxConnections = 3

func Multipath(localIPs []net.IP, proxyIP net.IP, remoteIPs []net.IP, packetChan chan []byte) error {
	dialers, err := createDialers(nil, localIPs, proxyIP)
	if err != nil {
		return err
	}

	localToProxyConn, proxyToServerConn, err := createConnections(dialers, remoteIPs)
	if err != nil {
		return err
	}
	err = sendMultipathData(localToProxyConn, proxyToServerConn, packetChan)
	if err != nil {
		return err
	}
	return nil
}

func sendMultipathData(localToProxyConn []UdpConnection, proxyToServerConn []UdpConnection, packetChan chan []byte) error {
	defer closeConnections(localToProxyConn)
	defer closeConnections(proxyToServerConn)

	var wg sync.WaitGroup

	for packet := range packetChan {
		for index := range localToProxyConn {
			wg.Add(1)
			go func(conn *UdpConnection, packet []byte) {
				defer wg.Done()
				conn.mu.Lock()
				defer conn.mu.Unlock()
				_, err := conn.conn.Write(packet)
				log.Printf("Sended the packet %v from %v\n", packet, conn.conn.RemoteAddr())
				if err != nil {
					log.Printf("Error writing to %v: %v\n", conn.conn.RemoteAddr(), err)
					return
				}
			}(&localToProxyConn[index], packet)
		}
		wg.Wait()
	}
	return nil
}

func createDialers(ZoneMap map[string]string, localIPs []net.IP, proxyIP net.IP) ([]net.Dialer, error) {
	dialers := make([]net.Dialer, 0)
	localIPs = append(localIPs, proxyIP) // the last one will be the proxy

	for _, ip := range localIPs {
		port := getRandomUDPPort()
		localAddr := &net.UDPAddr{IP: ip, Port: port}
		if ip.To4() == nil {
			localAddr.Zone = ZoneMap[ip.String()]
		}

		dialer := net.Dialer{
			LocalAddr: localAddr,
		}

		dialers = append(dialers, dialer)
	}

	return dialers, nil
}

func createConnections(dialers []net.Dialer, remoteIPs []net.IP) ([]UdpConnection, []UdpConnection, error) {
	localDialers := dialers[:len(dialers)-1]
	proxyDialer := dialers[len(dialers)-1]

	localToProxyConn := make([]UdpConnection, 0)
	proxyToServerConn := make([]UdpConnection, 0)

	for _, localDialer := range localDialers {
		conn, err := localDialer.Dial("udp", ListenAddr)
		if err != nil {
			closeConnections(localToProxyConn)
			return nil, nil, err
		}
		localToProxyConn = append(localToProxyConn, UdpConnection{mu: sync.Mutex{}, conn: conn})
	}

	for _, remoteIP := range remoteIPs {
		conn, err := proxyDialer.Dial("udp", remoteIP.String())
		if err != nil {
			closeConnections(localToProxyConn)
			closeConnections(proxyToServerConn)
			return nil, nil, err
		}
		proxyToServerConn = append(proxyToServerConn, UdpConnection{mu: sync.Mutex{}, conn: conn})
	}

	return localToProxyConn, proxyToServerConn, nil
}

func closeConnections(connections []UdpConnection) {
	for i := range connections {
		connections[i].conn.Close()
	}
}

func getRandomUDPPort() int {
	// Decide which range to use
	if rand.Intn(2) == 0 {
		// Range 5000–5500
		return 5000 + rand.Intn(551)
	} else {
		// Range 7000–8000
		return 7000 + rand.Intn(1001)
	}
}
