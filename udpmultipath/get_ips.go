package udpmultipath

import (
	"log"
	"net"
	"strings"
	"time"
)

const (
	InputFile    = "IpRanges/LAN.txt"
	OutputDir    = "IpRanges"
	OutputFile   = "discovered_lan_ips.txt"
	gameUDPPort  = 7263 // Can be changed or tested with a list of ports
	scanPerRange = 5    // Number of random IPs to test per subnet
	timeout      = 5 * time.Second
)

func GetLocalAddresses() ([]net.IP, []net.IP, error) {
	localIPv4 := make([]net.IP, 0)
	localIPv6 := make([]net.IP, 0)

	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, nil, err
	}

	for _, iface := range interfaces {
		// skip down interfaces or loopback
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		if strings.Contains(iface.Name, "vEthernet") || strings.Contains(iface.Name, "VirtualBox") {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			return nil, nil, err
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}

			if ip.To4() != nil {
				log.Printf("Found interface %v with IPv4\n", iface.Name)
				localIPv4 = append(localIPv4, ip)
			} else {
				log.Printf("Found interface %v with IPv6\n", iface.Name)
				localIPv6 = append(localIPv6, ip)
			}
		}
	}
	return localIPv4, localIPv6, nil
}
