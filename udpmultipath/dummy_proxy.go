package udpmultipath

import (
	"log"
	"net"
)

const (
	ListenAddr = "192.168.18.104:9000" // This will be your dummy "proxy" address
)

var (
	ListenIPString, ListenIPPort, _ = net.SplitHostPort(ListenAddr)
)

func ProxyServer() error {
	addr, err := net.ResolveUDPAddr("udp", ListenAddr)
	if err != nil {
		log.Fatalf("Failed to resolve UDP address: %v", err)
		return err
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", ListenAddr, err)
		return err
	}
	defer conn.Close()

	log.Printf("Dummy UDP proxy listening on %s", ListenAddr)

	buffer := make([]byte, 2048)
	for {
		n, remoteAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("Error reading from UDP: %v", err)
			continue
		}
		log.Printf("Received %d bytes from %v: %x", n, remoteAddr, buffer[:n])
	}
}

func DialProxyServer() error {
	address := &net.UDPAddr{
		IP: net.ParseIP("192.168.18.104"),
	}
	dialer := net.Dialer{LocalAddr: address}
	conn, err := dialer.Dial("udp", ListenAddr)
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.Write([]byte("Hello, UDP!"))
	return nil
}
