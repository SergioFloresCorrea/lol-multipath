package udpmultipath

import (
	"net"
)

const maxConnections = 3

func createDialers(ZoneMap map[string]string, localIPs []net.IP, proxyIP net.IP) ([]net.Dialer, error) {
	dialers := make([]net.Dialer, 0)
	localIPs = append(localIPs, proxyIP) // the last one will be the proxy

	for _, ip := range localIPs {
		localAddr := &net.UDPAddr{IP: ip, Port: 0}
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

func createConnections(dialers []net.Dialer, remoteIPs []net.IP) error {
	localDialers := dialers[:len(dialers)-1]
	proxyDialer := dialers[len(dialers)-1]
	proxyIP := proxyDialer.LocalAddr

	localToProxyConn := make([]net.Conn, 0)
	proxyToServerConn := make([]net.Conn, 0)

	for _, localDialer := range localDialers {
		conn, err := localDialer.Dial("udp", proxyIP.String())
		if err != nil {
			// Close anything that was already opened
			for _, c := range localToProxyConn {
				c.Close()
			}
			return err
		}
		localToProxyConn = append(localToProxyConn, conn)
	}

	for _, remoteIP := range remoteIPs {
		conn, err := proxyDialer.Dial("udp", remoteIP.String())
		if err != nil {
			for _, c := range localToProxyConn {
				c.Close()
			}
			for _, c := range proxyToServerConn {
				c.Close()
			}
			return err
		}
		proxyToServerConn = append(proxyToServerConn, conn)
	}

	return nil
}
