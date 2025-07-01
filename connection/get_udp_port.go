package connection

import (
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

// Checks for all available interfaces and matches those who have a corresponding local IPv4.
// Then, it tries to determine which of them is receiving the UDP traffic
// (i.e the interface with the lowest metric). Finally, it resolves the League Client Local IP address,
// Riot's remote server address (IP + Port) from the received UDP packets.
func GetRiotUDPAddressAndPort(port string, localIPv4 []net.IP) (string, string, error) {
	filter := fmt.Sprintf("udp and src port %s", port)
	possibleDevices := make([]string, 0)
	possibleIPs := make([]string, 0)

	// Get all devices (Npcap interfaces)
	devices, err := pcap.FindAllDevs()
	if err != nil {
		return "", "", fmt.Errorf("failed to find devices: %v", err)
	}

	// Match the device whose address is in localIPv4
	var device string

	for _, d := range devices {
		for _, addr := range d.Addresses {
			for _, ip := range localIPv4 {
				if addr.IP.Equal(ip) {
					device = d.Name
					possibleDevices = append(possibleDevices, device)
					possibleIPs = append(possibleIPs, addr.IP.String())
					break
				}
			}
		}
	}

	if len(possibleDevices) == 0 {
		return "", "", fmt.Errorf("no matching device found for provided local IPs")
	}

	if len(possibleDevices) != len(localIPv4) {
		return "", "", fmt.Errorf("an interface has no matching device")
	}

outer:
	for index, device := range possibleDevices {
		fmt.Printf("Using interface: %s\n", device)
		handle, err := pcap.OpenLive(device, 1024, false, timeout)
		if err != nil {
			return "", "", fmt.Errorf("failed to open device: %w", err)
		}
		if err := handle.SetBPFFilter(filter); err != nil {
			return "", "", fmt.Errorf("failed to set BPF filter: %w", err)
		}
		packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

		for {
			packet, err := packetSource.NextPacket()
			if err != nil {
				switch err {
				case pcap.NextErrorTimeoutExpired:
					handle.Close()
					fmt.Printf("Timeout on %s, trying next device\n", device)
					continue outer
				case io.EOF:
					handle.Close()
					continue outer
				default:
					handle.Close()
					return "", "", fmt.Errorf("packet read error on %s: %w", device, err)
				}
			}
			// we got a packet — process it:
			handle.Close()
			ipLayer := packet.NetworkLayer()
			udpLayer := packet.Layer(layers.LayerTypeUDP)
			if ipLayer != nil && udpLayer != nil {
				origIP := ipLayer.NetworkFlow().Src().String()
				dstIP := ipLayer.NetworkFlow().Dst().String()
				udp := udpLayer.(*layers.UDP)
				dstPort := strings.Split(udp.DstPort.String(), "(")[0]
				result := fmt.Sprintf("%s:%s", dstIP, dstPort)
				fmt.Printf("Captured destination: %s->%s\n", origIP, result)
				return result, possibleIPs[index], nil
			}
		}
	}

	return "", "", fmt.Errorf("no valid UDP packet found")
}
