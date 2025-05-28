package udpmultipath

import (
	"bytes"
	"encoding/gob"
	"log"
	"net"
	"time"

	"github.com/google/uuid"
)

const (
	ListenAddr = "192.168.18.104:9029" // This will be your dummy "proxy" address
)

func isDuplicate(id uuid.UUID, seenIDTracker *SeenIDTracker) bool {
	seenIDTracker.mu.Lock()
	defer seenIDTracker.mu.Unlock()
	if _, ok := seenIDTracker.SeenIDs[id]; ok {
		return true
	}
	seenIDTracker.SeenIDs[id] = time.Now()
	return false
}

func unwrapPacket(packet []byte) (*WrappedUDPPacket, error) {
	var wrappedPacket WrappedUDPPacket
	err := gob.NewDecoder(bytes.NewReader(packet)).Decode(&wrappedPacket)
	return &wrappedPacket, err
}

var (
	ListenIPString, ListenIPPort, _ = net.SplitHostPort(ListenAddr)
)

func ProxyServer(remoteIPs []net.IP, remotePort string) error {
	seenIDTracker := &SeenIDTracker{}
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

	buffer := make([]byte, 65535)
	for {
		go cleanupMap(seenIDTracker)
		n, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("Error reading from UDP: %v", err)
			continue
		}
		wrappedPacket, err := unwrapPacket(buffer[:n])
		if err != nil {
			log.Fatalf("Error unwrapping the UDP packet: %v", err)
			return err
		}

		if isDuplicate(wrappedPacket.ID, seenIDTracker) {
			continue
		}
		// log.Printf("Received %d bytes from %v", n, remoteAddr)

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
		}
	}
}

func cleanupMap(seenIDTracket *SeenIDTracker) {
	seenIDTracket.mu.Lock()
	defer seenIDTracket.mu.Unlock()
	now := time.Now()
	for id, timestamp := range seenIDTracket.SeenIDs {
		if now.Sub(timestamp) > 1*time.Second {
			delete(seenIDTracket.SeenIDs, id)
		}
	}
	return
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
