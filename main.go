package main

import (
	"flag"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/SergioFloresCorrea/bondcat-reduceping/connection"
	"github.com/SergioFloresCorrea/bondcat-reduceping/udpmultipath"
)

const remotePort = "7200"
const remotePortForBestSelection = 80

func main() {
	noProxy := flag.Bool("no-proxy", false, "run multipath without a proxy")
	flag.Parse()

	var udpConn connection.UDPResult

	done := make(chan struct{})
	tcpResultChan := make(chan connection.TCPResult, 1)
	udpResultChan := make(chan connection.UDPResult, 1)

	go connection.WaitForLeagueAndResolve("UDP", 1*time.Second, done, tcpResultChan, udpResultChan)

	<-done

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

	if err = udpmultipath.InterceptIncomingConnection(udpConn.LocalPort); err != nil {
		log.Fatalf("Couldn't re-inject incoming packets into the client: %v\n", err)
		os.Exit(1)
	}

	if *noProxy {
		portChan := make(chan []int)
		clientPort, _ := strconv.Atoi(udpConn.LocalPort)
		if err = udpmultipath.MultipathDirect(localIPv4, RiotIPPort, packetChan, portChan); err != nil {
			log.Fatalf("MultipathDirect failed: %v\n", err)
			os.Exit(1)
		}
		if err = udpmultipath.InterceptAndRewritePorts(portChan, clientPort); err != nil {
			log.Fatalf("Couldn't redirect incoming packets ")
		}
	} else {
		go udpmultipath.ProxyServer(remoteIPv4, riotPort, RiotLocalIP, udpConn.LocalPort) // dummy proxy

		if err = udpmultipath.InterceptOngoingConnection(udpConn.LocalPort, packetChan); err != nil {
			log.Fatalf("Couldn't intercept the connection %v\n", err)
			os.Exit(1)
		}

		err = udpmultipath.MultipathProxy(localIPv4, []string{udpmultipath.ProxyListenAddr}, packetChan)
		if err != nil {
			log.Fatalf("Couldn't make a multipath connection %v\n", err)
			os.Exit(1)
		}
	}

}
