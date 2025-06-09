package udpmultipath

import (
	"bufio"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
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

func GetRemoteAddresses(inputFile string) ([]string, error) {
	ranges, err := ReadIPRanges(inputFile)
	if err != nil {
		panic(fmt.Errorf("failed to read input file: %w", err))
	}

	var discovered []string

	for _, cidr := range ranges {
		ips, err := getRandomIPsInCIDR(cidr, scanPerRange)
		if err != nil {
			fmt.Printf("Skipping range %s: %v\n", cidr, err)
			continue
		}

		for _, ip := range ips {
			ok := testUDP(ip, gameUDPPort)
			if ok {
				entry := fmt.Sprintf("%s:%d", ip.String(), gameUDPPort)
				fmt.Printf("✓ Reachable: %s\n", entry)
				discovered = append(discovered, ip.String())
			} else {
				fmt.Printf("✗ Unreachable: %s:%d\n", ip.String(), gameUDPPort)
			}
		}
	}

	if err := writeDiscoveredIPs(discovered, OutputDir, OutputFile); err != nil {
		return nil, fmt.Errorf("failed to write output file: %w", err)
	}

	fmt.Printf("\nDone. %d IPs discovered and saved to %s/%s\n", len(discovered), OutputDir, OutputFile)
	return discovered, nil
}

func ReadIPRanges(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines, scanner.Err()
}

func getRandomIPsInCIDR(cidr string, count int) ([]net.IP, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}

	base := ipnet.IP.To4()
	if base == nil {
		return nil, fmt.Errorf("non-IPv4 range: %s", cidr)
	}

	var ips []net.IP
	mask := ipnet.Mask
	network := base.Mask(mask)

	for i := 0; i < count; i++ {
		randIP := make(net.IP, len(network))
		copy(randIP, network)
		randIP[3] = byte(rand.Intn(254) + 1) // avoid .0 and .255
		ips = append(ips, randIP)
	}

	return ips, nil
}

func testUDP(ip net.IP, port int) bool {
	addr := fmt.Sprintf("%s:%d", ip.String(), port)
	conn, err := net.DialTimeout("udp", addr, timeout)
	if err != nil {
		return false
	}
	defer conn.Close()

	// Send a probe packet
	_, err = conn.Write([]byte("ping"))
	if err != nil {
		return false
	}
	return true
}

func writeDiscoveredIPs(ips []string, dir, filename string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	f, err := os.Create(fmt.Sprintf("%s/%s", dir, filename))
	if err != nil {
		return err
	}
	defer f.Close()

	writer := bufio.NewWriter(f)
	for _, line := range ips {
		fmt.Fprintln(writer, line)
	}
	return writer.Flush()
}
