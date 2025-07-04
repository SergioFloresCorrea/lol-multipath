package udpmultipath

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/cespare/xxhash"
)

// Example proxy server. This program relies on the proxies having a really specific behaviour.
// They must listen for packets and, depending on whether the League Client or the League Server is sending them,
// Reroute the packets to the League Server and League Client respectively.
// The proxy server must also have a listener open for pings.
func (serverCfg *Config) ProxyServer(ctx context.Context, configCh chan ProxyConfig, ProxyListenAddr, ProxyPingListenAddr string) error {
	go func() {
		if err := PingHandler(ctx, ProxyPingListenAddr, serverCfg.Server, serverCfg.ServerMap); err != nil {
			log.Printf("ping handler failed: %v\n Closing the ping handler...", err)
			return
		}
	}()

	cfg := <-configCh

	remoteIP := cfg.RemoteIP
	remotePort := cfg.RemotePort
	clientIP := cfg.ClientIP
	clientPort := cfg.ClientPort

	remotePortInt, err := strconv.Atoi(remotePort)
	if err != nil {
		return fmt.Errorf("Remote Port must be a numeric string")
	}
	tracker := serverCfg.newTracker()

	addr, err := net.ResolveUDPAddr("udp", ProxyListenAddr)
	if err != nil {
		return fmt.Errorf("Failed to resolve UDP address: %w", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("Failed to listen on %s: %w", ProxyListenAddr, err)
	}
	defer conn.Close()

	clientAddr, err := net.ResolveUDPAddr(
		"udp",
		net.JoinHostPort(clientIP, strconv.Itoa(clientPort)),
	)
	if err != nil {
		return fmt.Errorf("Error in resolving the client's UDP address: %w", err)
	}

	log.Printf("Dummy UDP proxy listening on %s", ProxyListenAddr)

	buffer := make([]byte, 64*1024)

	ticker := time.NewTicker(tracker.cleanupInterval)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				tracker.cleanupHash()
			}
		}
	}()

	for {
		if err := ctx.Err(); err != nil {
			return nil
		}

		// avoid hanging for more than 1 second
		if err := conn.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
			log.Printf("unable to set read deadline: %v", err)
		}

		n, srcAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				if ctx.Err() != nil {
					return nil
				}
				continue
			}

			log.Printf("read error: %v", err)
			continue
		}

		hash := xxhash.Sum64(buffer[:n])
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

// Example ping handler. It listens to 8-byte udp packets and makes an HTTP request to the league servers
// (idea taken from https://pingtestlive.com/league-of-legends) and responds with the time (in ms) taken
// for the DNS resolution and TCP handshake to happen (we call it: the bloat).
func PingHandler(ctx context.Context, listenAddr, server string, serverMap map[string]string) error {
	pc, err := net.ListenPacket("udp", listenAddr)
	if err != nil {
		return err
	}
	defer pc.Close()
	log.Printf("Ping handler listening on %s for shard %s", listenAddr, server)

	reqBuf := make([]byte, 8) // interface's sent dummy

	for {
		if err := ctx.Err(); err != nil {
			return nil
		}

		n, addr, err := pc.ReadFrom(reqBuf)
		if err != nil {
			return fmt.Errorf("error in reading from the buffer: %w", err)
		}
		if n != 8 {
			return fmt.Errorf("unexpected number of received bytes. Received %d, Expected 8", n)
		}

		// measure HTTP bloat (total time - latency) from the proxy out to AWS
		bloat, err := GetBloat(serverMap, server)
		if err != nil {
			return fmt.Errorf("HTTP ping error (%s): %w", server, err)
		}

		bloatMs := bloat.Milliseconds()

		var buf bytes.Buffer
		if err := binary.Write(&buf, binary.BigEndian, bloatMs); err != nil {
			return fmt.Errorf("encode int64: %w", err)
		}

		// echo the 8-byte latency back to the client
		if _, err := pc.WriteTo(buf.Bytes(), addr); err != nil {
			return fmt.Errorf("failed to write ping echo: %w", err)
		}
	}
}
