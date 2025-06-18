package udpmultipath

import (
	"log"
	"math/rand"
	"net"
	"sync"
)

const maxConnections = 2

func Multipath(localIPs []net.IP, proxyIP net.IP, packetChan chan []byte) error {
	dialers, err := createDialers(nil, localIPs, proxyIP)
	if err != nil {
		return err
	}

	localToProxyConn, err := createConnections(dialers)
	if err != nil {
		return err
	}
	err = sendMultipathData(localToProxyConn, packetChan)
	if err != nil {
		return err
	}
	return nil
}

func sendMultipathData(localToProxyConn []UdpConnection, packetChan chan []byte) error {
	defer closeConnections(localToProxyConn)

	var wg sync.WaitGroup

	for packet := range packetChan {
		for index := range localToProxyConn {
			wg.Add(1)
			go func(conn *UdpConnection, wrappedPacket []byte) {
				defer wg.Done()
				conn.mu.Lock()
				defer conn.mu.Unlock()
				_, err := conn.conn.Write(wrappedPacket)
				// log.Printf("Sended the packet %v from %v\n", packet, conn.conn.RemoteAddr())
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

func createConnections(dialers []net.Dialer) ([]UdpConnection, error) {
	localDialers := dialers[:len(dialers)-1]

	localToProxyConn := make([]UdpConnection, 0)

	for _, localDialer := range localDialers {
		conn, err := localDialer.Dial("udp", ProxyListenAddr)
		if err != nil {
			closeConnections(localToProxyConn)
			return nil, err
		}
		localToProxyConn = append(localToProxyConn, UdpConnection{mu: sync.Mutex{}, conn: conn})
	}

	return localToProxyConn, nil
}

func closeConnections(connections []UdpConnection) {
	for i := range connections {
		connections[i].conn.Close()
	}
}

func getRandomUDPPort() int {
	if rand.Intn(2) == 0 {
		return 5000 + rand.Intn(551)
	} else {
		return 7000 + rand.Intn(1001)
	}
}
