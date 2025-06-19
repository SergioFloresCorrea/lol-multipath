package udpmultipath

import (
	"log"
	"net"
	"sync"
)

const maxConnections = 2

func MultipathProxy(localIPs []net.IP, proxyAddress []string, packetChan chan []byte) error {
	return multipath(localIPs, proxyAddress, packetChan, nil)
}

func MultipathDirect(localIPs []net.IP, remoteAddress string, packetChan chan []byte, portChan chan []int) error {
	return multipath(localIPs, []string{remoteAddress}, packetChan, portChan)
}

func multipath(localIPs []net.IP, targetsAddr []string, packetChan chan []byte, portChan chan []int) error {
	dialers, err := createDialers(localIPs)
	if err != nil {
		return err
	}

	localToTargetsConn, err := createConnections(dialers, targetsAddr)
	if err != nil {
		return err
	}

	if portChan != nil {
		portChan <- localToTargetsConn.Ports
	}
	err = sendMultipathData(localToTargetsConn.Conns, packetChan)
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
			go func(udpConn *UdpConnection, packet []byte) {
				defer wg.Done()
				udpConn.mu.Lock()
				defer udpConn.mu.Unlock()
				_, err := udpConn.conn.Write(packet)
				// log.Printf("Sended the packet %v from %v\n", packet, udpConn.conn.RemoteAddr())
				if err != nil {
					log.Printf("Error writing to %v: %v\n", udpConn.conn.RemoteAddr(), err)
					return
				}
			}(&localToProxyConn[index], packet)
		}
		wg.Wait()
	}
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

func createConnections(dialers []net.Dialer, targetsAddr []string) (ConnectionPort, error) {
	localToTargetsConn := ConnectionPort{}

	for _, localDialer := range dialers {
		for _, addr := range targetsAddr {
			conn, err := localDialer.Dial("udp", addr)
			if err != nil {
				closeConnections(localToTargetsConn.Conns)
				return ConnectionPort{}, err
			}
			lport := conn.LocalAddr().(*net.UDPAddr).Port
			localToTargetsConn.Conns = append(localToTargetsConn.Conns, UdpConnection{mu: sync.Mutex{}, conn: conn})
			localToTargetsConn.Ports = append(localToTargetsConn.Ports, lport)
		}
	}

	return localToTargetsConn, nil
}

func closeConnections(connections []UdpConnection) {
	for i := range connections {
		connections[i].conn.Close()
	}
}
