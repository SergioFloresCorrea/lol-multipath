package udpmultipath

import (
	"log"
	"net"
	"time"
)

const minRemoteIPs = 3

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

func GetRemoteAddresses(hostname string) ([]net.IP, []net.IP, error) {
	remoteIPv4 := make([]net.IP, 0)
	remoteIPv6 := make([]net.IP, 0)
	foundIPs := make(map[string]bool)
	counter := 0

	for len(remoteIPv4)+len(remoteIPv6) < minRemoteIPs {
		if counter > 10 { // waiting 10 seconds is a lot
			log.Printf("Only %d remote addresses of the %d could be found. Returning them.\n", len(remoteIPv4)+len(remoteIPv6), minRemoteIPs)
			break
		}
		ips, err := net.LookupIP(hostname)
		if err != nil {
			return nil, nil, err
		}

		for _, ip := range ips {
			if ip == nil {
				continue
			}

			ipName := ip.String()
			if ip.To4() != nil && !foundIPs[ipName] {
				remoteIPv4 = append(remoteIPv4, ip)
			} else {
				remoteIPv6 = append(remoteIPv6, ip)
			}
			foundIPs[ipName] = true
		}

		counter += 1
		time.Sleep(1 * time.Second) // avoid aggressive querying
	}
	return remoteIPv4, remoteIPv6, nil
}
