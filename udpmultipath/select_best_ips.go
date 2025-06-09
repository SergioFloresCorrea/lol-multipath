package udpmultipath

import (
	"fmt"
	"log"
	"math/rand/v2"
	"net"
	"sort"
	"strings"
	"time"
)

const (
	maxRemoteIPs = 3
)

func SelectRandomRemoteIPs(ips []string) ([]net.IP, error) {
	ipsLength := len(ips)
	if ipsLength == 0 {
		return nil, fmt.Errorf("no ips to select")
	}
	permutation := rand.Perm(ipsLength)
	idxSelection := permutation[:min(maxRemoteIPs, ipsLength)]

	var ipsSelected []net.IP
	for _, idx := range idxSelection {
		ipsSelected = append(ipsSelected, net.ParseIP(ips[idx]))
	}
	return ipsSelected, nil

}

func SelectBestRemoteIPs(ips []string, port int) ([]string, error) {
	var results []IpLatency
	for _, ip := range ips {
		latency, ok := testTCP(ip, port)
		if !ok {
			continue
		}
		results = append(results, IpLatency{ip: ip, latency: latency})
		fmt.Printf("Ip: %v, Latency: %v\n", ip, latency)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("couldn't establish a TCP connection with any of the IPs")
	}

	// ascending order, less latency goes on lower index
	sort.Slice(results, func(i, j int) bool {
		return results[i].latency < results[j].latency
	})

	var best []string
	for i := 0; i < min(maxRemoteIPs, len(results)); i++ {
		best = append(best, results[i].ip)
	}

	return best, nil

}

func testTCP(ip string, port int) (time.Duration, bool) {
	log.Printf("Testing connection for IP: %v", ip)
	addr := fmt.Sprintf("%s:%d", ip, port)
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, timeout)
	elapsed := time.Since(start)

	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return elapsed, true // received an answer
		} else {
			log.Printf("%v\n", err)
		}
		return 0, false // unreachable
	}
	conn.Close()
	return elapsed, true // successful connect
}
