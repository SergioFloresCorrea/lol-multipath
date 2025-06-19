package udpmultipath

import (
	"crypto/sha256"
	"log"
	"net"
	"os"
	"strconv"
	"time"
)

const (
	ProxyListenAddr = "192.168.18.104:9029"
	ProxyListenPort = "9029"
	cleanupInterval = 1 * time.Second
)

var (
	ListenIPString, ListenIPPort, _ = net.SplitHostPort(ProxyListenAddr)
)

func ProxyServer(remoteIP net.IP, remotePort, clientIP, clientPort string) {
	remotePortInt, err := strconv.Atoi(remotePort)
	if err != nil {
		log.Fatalf("Remote Port must be a string")
		os.Exit(1)
	}
	tracker := &SeenHashTracker{SeenHash: make(map[[32]byte]time.Time)}

	addr, err := net.ResolveUDPAddr("udp", ProxyListenAddr)
	if err != nil {
		log.Fatalf("Failed to resolve UDP address: %v", err)
		os.Exit(1)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", ProxyListenAddr, err)
		os.Exit(1)
	}
	defer conn.Close()

	clientAddr, _ := net.ResolveUDPAddr(
		"udp",
		net.JoinHostPort(clientIP, clientPort),
	)

	log.Printf("Dummy UDP proxy listening on %s", ProxyListenAddr)

	buffer := make([]byte, 64*1024)

	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	go func(ticker *time.Ticker) {
		for range ticker.C {
			tracker.cleanupHash()
		}
	}(ticker)

	for {
		n, srcAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("read error: %v", err)
			continue
		}

		hash := sha256.Sum256(buffer[:n])
		if tracker.isHashDuplicate(hash) {
			continue
		}

		isFromRemote := false
		if srcAddr.IP.Equal(remoteIP) && srcAddr.Port == remotePortInt {
			isFromRemote = true
		}

		if isFromRemote {
			// Redirect into the client’s real UDP port
			if _, err := conn.WriteToUDP(buffer[:n], clientAddr); err != nil {
				log.Printf("failed to send back to client: %v", err)
			}
		} else {
			// New outgoing packet: fan‐out to remote IP
			raddr := &net.UDPAddr{IP: remoteIP, Port: remotePortInt}
			if _, err := conn.WriteToUDP(buffer[:n], raddr); err != nil {
				log.Printf("failed to forward to %v: %v", raddr, err)
			}
		}
	}
}
