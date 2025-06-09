package connection

import (
	"fmt"
	"net"
	"strings"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

const (
	snapshotLen = 1024
	promiscuous = false
	timeout     = pcap.BlockForever
)

func GetRiotUDPAddressAndPort(port string, localIPv4 []net.IP) (string, error) {
	filter := fmt.Sprintf("udp and src port %s", port)

	// Get all devices (Npcap interfaces)
	devices, err := pcap.FindAllDevs()
	if err != nil {
		return "", fmt.Errorf("failed to find devices: %v", err)
	}

	// Match the device whose address is in localIPv4
	var device string
DEVICE_LOOP:
	for _, d := range devices {
		for _, addr := range d.Addresses {
			for _, ip := range localIPv4 {
				if addr.IP.Equal(ip) {
					device = d.Name
					break DEVICE_LOOP
				}
			}
		}
	}

	if device == "" {
		return "", fmt.Errorf("no matching device found for provided local IPs")
	}
	fmt.Printf("Using interface: %s\n", device)

	// Open the device for live capture
	handle, err := pcap.OpenLive(device, 1024, false, pcap.BlockForever)
	if err != nil {
		return "", fmt.Errorf("failed to open device: %v", err)
	}
	defer handle.Close()

	// Apply BPF filter
	if err := handle.SetBPFFilter(filter); err != nil {
		return "", fmt.Errorf("failed to set BPF filter: %v", err)
	}
	fmt.Printf("Listening for UDP packets from port %s...\n", port)

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

	// Return the first valid remote IP and port we find
	for packet := range packetSource.Packets() {
		// Ensure packet has IP and UDP layers
		ipLayer := packet.NetworkLayer()
		udpLayer := packet.Layer(layers.LayerTypeUDP)

		if ipLayer != nil && udpLayer != nil {
			dstIP := ipLayer.NetworkFlow().Dst().String()
			udp := udpLayer.(*layers.UDP)
			dstPort := strings.Split(udp.DstPort.String(), "(")[0]

			result := fmt.Sprintf("%s:%s", dstIP, dstPort)
			fmt.Printf("Captured destination: %s\n", result)
			return result, nil
		}
	}

	return "", fmt.Errorf("no valid UDP packet found")
}
