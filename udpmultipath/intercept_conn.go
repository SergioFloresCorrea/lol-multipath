package udpmultipath

import (
	"crypto/sha256"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/lysShub/divert-go"
	"github.com/pkg/errors"
	"golang.org/x/sys/windows"
)

// for proxy
func InterceptOngoingConnection(port string, packetChan chan<- []byte) error {
	_ = divert.MustLoad(divert.DLL)
	filter := fmt.Sprintf("udp.SrcPort == %s and outbound and !loopback", port)
	h, err := divert.Open(filter, divert.Network, 0, 0)
	if err != nil {
		return fmt.Errorf("failed to open outbound divert handle: %w", err)
	}

	go func() {
		defer h.Close()
		buf := make([]byte, 64*1024)
		var addr divert.Address

		for {
			n, err := h.Recv(buf, &addr)
			if err != nil {
				if errors.Is(err, windows.ERROR_INSUFFICIENT_BUFFER) {
					continue
				}
				log.Printf("intercept ongoing error: %v", err)
				close(packetChan)
				return
			}
			if n == 0 {
				continue
			}

			// log.Printf("Intercepted package of length %d", n)

			pkt := buf[:n]
			/*
				if _, err := h.Send(pkt, &addr); err != nil {
					log.Printf("Failed to reinject original outbound packet: %v", err)
				}
			*/

			p := gopacket.NewPacket(pkt, layers.LayerTypeIPv4, gopacket.Default)
			if udpLayer := p.Layer(layers.LayerTypeUDP); udpLayer != nil {
				udp := udpLayer.(*layers.UDP)
				packetChan <- udp.Payload
			}
		}
	}()

	return nil
}

func InterceptIncomingConnection(port string) error {
	tracker := &SeenHashTracker{SeenHash: make(map[[32]byte]time.Time)}
	_ = divert.MustLoad(divert.DLL)
	filter := fmt.Sprintf("udp.DstPort == %s and inbound and !loopback", port)
	// filter := fmt.Sprintf("tcp.DstPort == %s and inbound and !loopback", port)
	h, err := divert.Open(filter, divert.Network, 0, 0)
	if err != nil {
		return fmt.Errorf("failed to open inbound divert handle: %w", err)
	}

	// Periodically clean outdated hashes
	ticker := time.NewTicker(cleanupInterval)
	go func() {
		for range ticker.C {
			tracker.cleanupHash()
		}
	}()

	go func() {
		defer func() {
			ticker.Stop()
			h.Close()
		}()

		buf := make([]byte, 64*1024)
		var addr divert.Address

		for {
			n, err := h.Recv(buf, &addr)
			if err != nil {
				if errors.Is(err, windows.ERROR_INSUFFICIENT_BUFFER) {
					continue
				}
				log.Printf("Error %v\n", err)
				return
			}
			if n == 0 {
				log.Printf("No packets :(")
				continue
			}

			// log.Printf("Received package of length %d", n)
			pkt := make([]byte, n)
			copy(pkt, buf[:n])

			var hash [32]byte
			packet := gopacket.NewPacket(pkt, layers.LayerTypeIPv4, gopacket.Default)
			udpLayer := packet.Layer(layers.LayerTypeUDP)
			if udpLayer != nil {
				udp := udpLayer.(*layers.UDP)
				hash = sha256.Sum256(udp.Payload)

				if !tracker.isHashDuplicate(hash) {
					if _, err := h.Send(pkt, &addr); err != nil {
						log.Printf("reinject failed: %v", err)
					}
				} else {
					log.Printf("Duplicate packet detected — multipath likely working")
				}
			}
		}
	}()

	return nil
}

// no proxy
func InterceptAndRewritePorts(portChan chan []int, clientPort int) error {
	/*
		filter like: "udp and inbound and (udp.DstPort == XXX
		or udp.DstPort == YYY) and !loopback"
	*/
	ports := <-portChan
	cond := make([]string, len(ports))
	for i, p := range ports {
		cond[i] = fmt.Sprintf("udp.DstPort == %d", p)
	}
	filter := fmt.Sprintf("udp and inbound and (%s) and !loopback",
		strings.Join(cond, " or "))

	h, err := divert.Open(filter, divert.Network, 0, 0)
	if err != nil {
		return err
	}
	go func() {
		defer h.Close()
		buf := make([]byte, 64*1024)
		var addr divert.Address
		for {
			n, err := h.Recv(buf, &addr)
			if err != nil {
				continue
			}
			pkt := buf[:n]
			p := gopacket.NewPacket(pkt, layers.LayerTypeIPv4, gopacket.Default)
			ip4 := p.Layer(layers.LayerTypeIPv4).(*layers.IPv4)
			udp := p.Layer(layers.LayerTypeUDP).(*layers.UDP)

			udp.DstPort = layers.UDPPort(clientPort)
			udp.SetNetworkLayerForChecksum(ip4)

			out := gopacket.NewSerializeBuffer()
			opts := gopacket.SerializeOptions{
				FixLengths: true, ComputeChecksums: true,
			}
			gopacket.SerializeLayers(out, opts, ip4, udp,
				gopacket.Payload(udp.Payload))
			if _, err = h.Send(out.Bytes(), &addr); err != nil {
				log.Printf("reinject failed: %v", err)
			}
		}
	}()
	return nil
}
