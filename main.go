package main

import (
	"flag"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/SergioFloresCorrea/bondcat-reduceping/connection"
	"github.com/SergioFloresCorrea/bondcat-reduceping/udpmultipath"
)

func main() {
	proxyListenCSV := flag.String("proxy-listen-addr", "", "comma-separated list of proxy listen addresses (e.g. \"A:9029,B:9030\")")
	proxyPingCSV := flag.String("proxy-ping-listen-addr", "", "comma-separated list of proxy ping addresses   (e.g. \"A:10001,B:10002\")")
	dynamicMode := flag.Bool("dynamic", true, "enable periodic proxy reselection")
	flag.Parse()

	proxyListenAddrs := parseCSV(*proxyListenCSV)
	proxyPingAddrs := parseCSV(*proxyPingCSV)

	if len(proxyListenAddrs) != len(proxyPingAddrs) {
		log.Fatalf("need same number of --proxy-listen-addr (%d) and --proxy-ping-listen-addr (%d)", len(proxyListenAddrs), len(proxyPingAddrs))
	}

	var udpConn connection.UDPResult

	done := make(chan struct{})
	tcpResultChan := make(chan connection.TCPResult, 1)
	udpResultChan := make(chan connection.UDPResult, 1)

	go connection.WaitForLeagueAndResolve("UDP", 1*time.Second, done, tcpResultChan, udpResultChan)

	<-done

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

	select {
	case result := <-tcpResultChan:
		if result.Err == nil {
			log.Println("TCP Done!")
			log.Println("Remote IPs:", result.RemoteIPs)
			log.Println("Local IPs:", result.LocalIPs)
		}
	case result := <-udpResultChan:
		if result.Err == nil {
			log.Println("UDP Done!")
			log.Println("Local Port:", result.LocalPort)
			udpConn = result
		}
	}

	packetChan := make(chan []byte)

	localIPv4, localIPv6, err := udpmultipath.GetLocalAddresses()
	if err != nil {
		log.Fatalf("%v\n", err)
		os.Exit(1)
	}
	if len(localIPv4) == 0 {
		log.Fatalf("No local interfaces with IPv4 could be found.")
		os.Exit(1)
	}

	log.Printf("Local Interface IPv4 addresses: %v", localIPv4)
	log.Printf("Local Interface IPv6 addresses (unused): %v", localIPv6)

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

	remoteIPv4 := net.ParseIP(riotIP) // testing
	log.Printf("Found Riot IP and Port: %v, %v", riotIP, riotPort)

	if err = udpmultipath.InterceptOngoingConnection(udpConn.LocalPort, packetChan); err != nil {
		log.Fatalf("Couldn't intercept ongoing packets from the client: %v\n", err)
		os.Exit(1)
	}

	if len(proxyListenAddrs) == 0 && len(proxyPingAddrs) == 0 {
		portChan := make(chan []int, 1)
		clientPort, _ := strconv.Atoi(udpConn.LocalPort)

		go func() {
			if err = udpmultipath.MultipathDirect(localIPv4, RiotIPPort, packetChan, portChan); err != nil {
				log.Fatalf("MultipathDirect failed: %v\n", err)
				os.Exit(1)
			}
		}()

		if err = udpmultipath.InterceptAndRewritePorts(portChan, clientPort); err != nil {
			log.Fatalf("Couldn't redirect incoming packets ")
		}

		select {}
	} else {
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
				if err := udpmultipath.ProxyServer(proxyConfigCh, listen, ping, "LAN"); err != nil {
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

		err = udpmultipath.MultipathProxy(localIPv4, proxyListenAddrs, proxyPingAddrs, packetChan, *dynamicMode)
		if err != nil {
			log.Fatalf("Couldn't make a multipath connection %v\n", err)
			os.Exit(1)
		}
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
