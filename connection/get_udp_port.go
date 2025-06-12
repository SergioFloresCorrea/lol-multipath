package connection

import (
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

const (
	snapshotLen = 1024
	promiscuous = false
	timeout     = 2 * time.Second
)

func GetRiotUDPAddressAndPort(port string, localIPv4 []net.IP) (string, error) {
	filter := fmt.Sprintf("udp and src port %s", port)
	possibleDevices := make([]string, 0)

	// Get all devices (Npcap interfaces)
	devices, err := pcap.FindAllDevs()
	if err != nil {
		return "", fmt.Errorf("failed to find devices: %v", err)
	}

	// Match the device whose address is in localIPv4
	var device string

	for _, d := range devices {
		for _, addr := range d.Addresses {
			for _, ip := range localIPv4 {
				if addr.IP.Equal(ip) {
					device = d.Name
					possibleDevices = append(possibleDevices, device)
				}
			}
		}
	}

	if len(possibleDevices) == 0 {
		return "", fmt.Errorf("no matching device found for provided local IPs")
	}

	if len(possibleDevices) != len(localIPv4) {
		log.Fatalf("An interface has no matching device")
	}

outer:
	for _, device := range possibleDevices {
		fmt.Printf("Using interface: %s\n", device)
		handle, err := pcap.OpenLive(device, 1024, false, timeout)
		if err != nil {
			return "", fmt.Errorf("failed to open device: %v", err)
		}
		defer handle.Close()
		if err := handle.SetBPFFilter(filter); err != nil {
			return "", fmt.Errorf("failed to set BPF filter: %v", err)
		}
		packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

		for {
			packet, err := packetSource.NextPacket()
			if err != nil {
				switch err {
				case pcap.NextErrorTimeoutExpired:
					fmt.Printf("Timeout on %s, trying next device\n", device)
					continue outer
				case io.EOF:
					// unlikely for a live capture, but just in case
					continue outer
				default:
					return "", fmt.Errorf("packet read error on %s: %v", device, err)
				}
			}
			// we got a packet — process it:
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
	}

	return "", fmt.Errorf("no valid UDP packet found")
}
