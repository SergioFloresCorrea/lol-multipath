package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/SergioFloresCorrea/bondcat-reduceping/connection"
	"github.com/SergioFloresCorrea/bondcat-reduceping/udpmultipath"
)

func main() {
	proxyListenCSV := flag.String("proxy-listen-addr", "", "comma-separated list of proxy listen addresses (e.g. \"A:9029,B:9030\")")
	proxyPingCSV := flag.String("proxy-ping-listen-addr", "", "comma-separated list of proxy ping addresses   (e.g. \"A:10001,B:10002\")")
	server := flag.String("server", "", "league of legends server. Available servers: NA, LAS, EUW, OCE, EUNE, RU, TR, JP, KR")
	thresholdFactor := flag.Float64("threshold-factor", 1.4, "exclude connections whose ping exceeds thresholdFactorxthe lowest observed ping")
	updateInterval := flag.Duration("update-interval", 30*time.Second, "interval at which to refresh each connection's ping metrics")
	probeInterval := flag.Duration("probe-interval", 10*time.Second, "interval at which to probe for down connections")
	timeout := flag.Duration("timeout", 1*time.Second, "ping response timeout")
	maxConnections := flag.Int("max-connections", 2, "maximum number of connections for multipath routing")
	dynamicMode := flag.Bool("dynamic", false, "enable periodic proxy reselection")

	flag.Parse()

	if *proxyListenCSV == "" {
		log.Printf("Error: -proxy-listen-addr is required")
		flag.Usage()
		os.Exit(2)
	}

	proxyListenAddrs := parseCSV(*proxyListenCSV)
	proxyPingAddrs := parseCSV(*proxyPingCSV)

	if len(proxyListenAddrs) != len(proxyPingAddrs) {
		log.Fatalf("need same number of --proxy-listen-addr (%d) and --proxy-ping-listen-addr (%d)", len(proxyListenAddrs), len(proxyPingAddrs))
	} else if len(proxyListenAddrs) == 0 {
		log.Printf("need at least one proxy for the script to be of use.")
		os.Exit(1)
	}

	if *server == "" {
		log.Printf("Error: -server is required")
		flag.Usage()
		os.Exit(2)
	}

	RAND, _ := randomHex(5)
	cfg := udpmultipath.Config{
		Rand:            RAND,
		Server:          strings.ToUpper(*server),
		UpdateInterval:  *updateInterval,
		ProbeInterval:   *probeInterval,
		ThresholdFactor: *thresholdFactor,
		Timeout:         *timeout,
		MaxConnections:  *maxConnections,
		Dynamic:         *dynamicMode,
	}

	cfg.GenerateServerMap()

	// Create a global context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigs
		log.Println("interrupt received, shutting down…")
		cancel()
	}()

	resultCh := make(chan connection.UDPResult, 1)
	go func() {
		res, err := connection.WaitForLeagueAndResolve(ctx, 1*time.Second)
		if err != nil {
			cancel()
			log.Fatalf("failed to resolve league process: %v", err)
		}
		resultCh <- res
	}()

	/*
		go func() {
			for {
				active := connection.CheckIfLeagueIsActive()
				if !active {
					log.Println("League process has exited. Shutting down...")
					os.Exit(0)
				}
				time.Sleep(10 * time.Second)
			}
		}()
	*/
	udpConn := <-resultCh
	log.Println("UDP Done!")
	log.Println("Local Port:", udpConn.LocalPort)

	packetChan := make(chan []byte)

	localIPv4, err := udpmultipath.GetLocalAddresses()
	if err != nil {
		log.Fatalf("%v\n", err)
		os.Exit(1)
	}
	if len(localIPv4) == 0 {
		log.Fatalf("No local interfaces with IPv4 could be found.")
		os.Exit(1)
	}

	log.Printf("Local Interface IPv4 addresses: %v", redactIPs(localIPv4))

	RiotIPPort, RiotLocalIP, err := connection.GetRiotUDPAddressAndPort(udpConn.LocalPort, localIPv4)
	if err != nil {
		log.Fatalf("%v\n", err)
		os.Exit(1)
	}
	riotIP, riotPort, err := net.SplitHostPort(RiotIPPort)
	if err != nil {
		log.Fatalf("%v\n", err)
		os.Exit(1)
	}

	remoteIPv4 := net.ParseIP(riotIP)
	log.Printf("Found Riot IP and Port: %v, %v", riotIP, riotPort)

	if err = udpmultipath.InterceptOngoingConnection(udpConn.LocalPort, packetChan); err != nil {
		log.Fatalf("Couldn't intercept ongoing packets from the client: %v\n", err)
		os.Exit(1)
	}

	centralCh := make(chan udpmultipath.ProxyConfig)

	proxyChs := make([]chan udpmultipath.ProxyConfig, len(proxyListenAddrs))
	for i := range proxyListenAddrs {
		proxyChs[i] = make(chan udpmultipath.ProxyConfig)
	}

	for i, listen := range proxyListenAddrs {
		ping := proxyPingAddrs[i]
		cfgCh := proxyChs[i]
		go func(listen, ping string, proxyConfigCh chan udpmultipath.ProxyConfig) {
			// for testing only, ideally, you would have already setup these servers
			if err := cfg.ProxyServer(ctx, proxyConfigCh, listen, ping); err != nil {
				log.Fatalf("%v\n", err)
				os.Exit(1)
			}
		}(listen, ping, cfgCh)
	}

	go broadcast(centralCh, proxyChs)

	centralCh <- udpmultipath.ProxyConfig{
		RemoteIP:   remoteIPv4,
		RemotePort: riotPort,
		ClientIP:   RiotLocalIP,
		ClientPort: udpConn.LocalPort,
	}

	close(centralCh)

	err = cfg.MultipathProxy(ctx, localIPv4, proxyListenAddrs, proxyPingAddrs, packetChan)
	if err != nil {
		log.Fatalf("Couldn't make a multipath connection %v\n", err)
		os.Exit(1)
	}

}

func parseCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func broadcast(in <-chan udpmultipath.ProxyConfig, outs []chan udpmultipath.ProxyConfig) {
	for cfg := range in {
		for _, out := range outs {
			out <- cfg
		}
	}
}

// Only works for IPv4. Masks the IPs to the form 192.0.2.x
func redactIPs(ips []net.IP) []string {
	redactedIPs := make([]string, len(ips))
	for index, ip := range ips {
		ipString := ip.String()
		ipParts := strings.Split(ipString, ".")
		ipParts[len(ipParts)-1] = "x"
		redactedIPs[index] = strings.Join(ipParts, ".")
	}
	return redactedIPs
}

func randomHex(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
