package udpmultipath

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"
)

const (
	cleanupInterval = 1 * time.Second
)

func ProxyServer(configCh chan ProxyConfig, ProxyListenAddr, ProxyPingListenAddr, server string) error {
	go func() {
		if err := PingHandler(ProxyPingListenAddr, server); err != nil {
			log.Fatalf("ping handler failed: %v", err)
		}
	}()

	cfg := <-configCh

	remoteIP := cfg.RemoteIP
	remotePort := cfg.RemotePort
	clientIP := cfg.ClientIP
	clientPort := cfg.ClientPort

	remotePortInt, err := strconv.Atoi(remotePort)
	if err != nil {
		return fmt.Errorf("Remote Port must be a string")
	}
	tracker := &SeenHashTracker{SeenHash: make(map[[32]byte]time.Time)}

	addr, err := net.ResolveUDPAddr("udp", ProxyListenAddr)
	if err != nil {
		return fmt.Errorf("Failed to resolve UDP address: %v", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("Failed to listen on %s: %v", ProxyListenAddr, err)
	}
	defer conn.Close()

	clientAddr, err := net.ResolveUDPAddr(
		"udp",
		net.JoinHostPort(clientIP, clientPort),
	)
	if err != nil {
		return fmt.Errorf("Error in resolving the client's UDP address: %v", err)
	}

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

func PingHandler(listenAddr, server string) error {
	pc, err := net.ListenPacket("udp", listenAddr)
	if err != nil {
		return err
	}
	defer pc.Close()
	log.Printf("Ping handler listening on %s for shard %s", listenAddr, server)

	reqBuf := make([]byte, 8)  // interface's sent dummy
	respBuf := make([]byte, 8) // to hold the 8-byte nanosecond latency

	for {
		n, addr, err := pc.ReadFrom(reqBuf)
		if err != nil {
			return fmt.Errorf("error in reading from the buffer: %v", err)
		}
		if n != 8 {
			return fmt.Errorf("unexpected number of received bytes. Received %d, Expected 8", n)
		}

		// measure HTTP “ping” from the proxy out to AWS
		bloat, err := GetBloat(server)
		if err != nil {
			return fmt.Errorf("HTTP ping error (%s): %v", server, err)
		}

		binary.BigEndian.PutUint64(respBuf, uint64(bloat.Milliseconds()))

		// echo the 8-byte latency back to the client
		if _, err := pc.WriteTo(respBuf, addr); err != nil {
			log.Printf("failed to write ping echo: %v", err)
		}
	}
}
