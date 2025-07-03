package udpmultipath

import (
	"errors"
	"log"
	"net"
	"strings"
)

// Checks for every interface that is not down and not a loopback interface. It also filters
// any virtual interfaces as they are assumed to not be able to communicate with the League Client and League Servers.
// It returns the Ipv4 of the remaining interfaces, and any errors that may arise.
func GetLocalAddresses() ([]net.IP, error) {
	localIPv4 := make([]net.IP, 0)

	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
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
			return nil, err
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

			addrRedacted, err := redactAddress(addr.String())
			if err != nil {
				continue
			}

			if ip.To4() != nil {
				log.Printf("Found interface %v with IPv4: %s\n", iface.Name, addrRedacted)
				localIPv4 = append(localIPv4, ip)
			}
		}
	}
	return localIPv4, nil
}

// Redacts an address string to 192.0.1.x/x. Only works for IPv4
func redactAddress(addr string) (string, error) {
	parts := strings.SplitN(addr, "/", 2)
	ipStringParts := strings.Split(parts[0], ".")
	if len(ipStringParts) != 4 {
		return "", errors.New("invalid IPv4 address: " + parts[0])
	}
	ipStringParts[3] = "x"
	parts[1] = "x"

	redacted := strings.Join(ipStringParts, ".")
	if len(parts) == 2 {
		redacted += "/" + parts[1]
	}

	return redacted, nil
}
