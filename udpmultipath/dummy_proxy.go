package udpmultipath

import (
	"bytes"
	"encoding/gob"
	"log"
	"net"
	"os"
	"time"

	"github.com/google/uuid"
)

const (
	ProxyListenAddr = "192.168.18.104:9029"
	ProxyListenPort = "9029"
	cleanupInterval = 1 * time.Second
)

func unwrapPacket(packet []byte) (*WrappedUDPPacket, error) {
	var wrappedPacket WrappedUDPPacket
	err := gob.NewDecoder(bytes.NewReader(packet)).Decode(&wrappedPacket)
	return &wrappedPacket, err
}

var (
	ListenIPString, ListenIPPort, _ = net.SplitHostPort(ProxyListenAddr)
)

func ProxyServer(remoteIPs []net.IP, remotePort string) error {
	seenIDTracker := &SeenIDTracker{SeenIDs: make(map[uuid.UUID]time.Time)}
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

	log.Printf("Dummy UDP proxy listening on %s", ProxyListenAddr)

	buffer := make([]byte, 64*1024)

	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	go func(ticker *time.Ticker) {
		for range ticker.C {
			seenIDTracker.cleanupTracker()
		}
	}(ticker)

	for {
		n, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("Error reading from UDP: %v", err)
			continue
		}
		wrappedPacket, err := unwrapPacket(buffer[:n])
		if err != nil {
			log.Fatalf("Error unwrapping the UDP packet: %v", err)
			os.Exit(1)
		}

		if seenIDTracker.isDuplicate(wrappedPacket.ID) {
			continue
		}
		// log.Printf("Received %d bytes", n)

		for _, ip := range remoteIPs {
			remoteAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(ip.String(), remotePort))
			if err != nil {
				log.Printf("Resolve error: %v", err)
				continue
			}
			_, err = conn.WriteToUDP(wrappedPacket.Data, remoteAddr)
			if err != nil {
				log.Printf("Forward error: %v", err)
			}
			// log.Printf("Sent %d butes to %v", len(wrappedPacket.Data), remoteAddr)
		}
	}
}

func (tracker *SeenIDTracker) cleanupTracker() {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	now := time.Now()
	for id, timestamp := range tracker.SeenIDs {
		if now.Sub(timestamp) > cleanupInterval {
			delete(tracker.SeenIDs, id)
		}
	}
	return
}

func (tracker *SeenIDTracker) isDuplicate(id uuid.UUID) bool {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	if _, ok := tracker.SeenIDs[id]; ok {
		return true
	}
	tracker.SeenIDs[id] = time.Now()
	return false
}

func DialProxyServer() error {
	address := &net.UDPAddr{
		IP: net.ParseIP("192.168.18.104"),
	}
	dialer := net.Dialer{LocalAddr: address}
	conn, err := dialer.Dial("udp", ProxyListenAddr)
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.Write([]byte("Hello, UDP!"))
	return nil
}
