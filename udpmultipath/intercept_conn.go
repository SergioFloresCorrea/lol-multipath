package udpmultipath

import (
	"fmt"
	"log"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/lysShub/divert-go"
	"github.com/pkg/errors"
	"golang.org/x/sys/windows"
)

// Intercepts the connection going out from `port` and redirects it into `packetChan`
// without re-introducing the packet into the network stack
func InterceptOngoingConnection(port int, packetChan chan<- []byte) error {
	_ = divert.MustLoad(divert.DLL)
	filter := fmt.Sprintf("udp.SrcPort == %d and outbound and !loopback", port)
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

			pkt := buf[:n]

			p := gopacket.NewPacket(pkt, layers.LayerTypeIPv4, gopacket.Default)
			if udpLayer := p.Layer(layers.LayerTypeUDP); udpLayer != nil {
				udp := udpLayer.(*layers.UDP)
				packetChan <- udp.Payload
			}
		}
	}()

	return nil
}
