package udpmultipath

import (
	"log"
	"net"
	"strings"
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
				log.Printf("Found interface %v with IPv4: %s\n", iface.Name, addr.String())
				localIPv4 = append(localIPv4, ip)
			} else {
				log.Printf("Found interface %v with IPv6: %s\n", iface.Name, addr.String())
				localIPv6 = append(localIPv6, ip)
			}
		}
	}
	return localIPv4, localIPv6, nil
}
