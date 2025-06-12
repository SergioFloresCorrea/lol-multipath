package main

import (
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/SergioFloresCorrea/bondcat-reduceping/connection"
	"github.com/SergioFloresCorrea/bondcat-reduceping/udpmultipath"
)

const hostname = "la1.api.riotgames.com"
const remotePort = "5100"
const remotePortForBestSelection = 80

func main() {
	/*
		discoveredIP, err := udpmultipath.GetRemoteAddresses()
		if err != nil {
			log.Fatalf("%v\n", err)
			os.Exit(1)
		}

		bestIPs, err := udpmultipath.SelectBestRemoteIPs(discoveredIP, remotePortForBestSelection)
		if err != nil {
			log.Fatalf("%v\n", err)
			os.Exit(1)
		}
		log.Printf("Best Ips found: %v", bestIPs)
	*/

	discoveredIP, err := udpmultipath.ReadIPRanges(filepath.Join(udpmultipath.OutputDir, udpmultipath.OutputFile))

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
			log.Println("Local IP:", result.LocalIP)
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

	remoteIPv4, err := udpmultipath.SelectRandomRemoteIPs(discoveredIP)
	if err != nil {
		log.Fatalf("%v\n", err)
		os.Exit(1)
	}

	log.Printf("Local Interface IPv4 addresses: %v", localIPv4)
	log.Printf("Local Interface IPv6 addresses: %v", localIPv6)

	log.Printf("Remote %v IPv4 addresses: %v", hostname, remoteIPv4)

	RiotIPPort, err := connection.GetRiotUDPAddressAndPort(udpConn.LocalPort, localIPv4)
	if err != nil {
		log.Fatalf("%v\n", err)
		os.Exit(1)
	}
	riotIP, riotPort, err := net.SplitHostPort(RiotIPPort)
	if err != nil {
		log.Fatalf("%v\n", err)
		os.Exit(1)
	}
	log.Printf("Found Riot IP and Port: %v, %v", riotIP, riotPort)

	// remoteIPv4 = []net.IP{net.ParseIP(riotIP)} // testing

	proxyIP := net.ParseIP(udpmultipath.ListenIPString) // or the IP where dummy_proxy is listening
	go udpmultipath.ProxyServer(remoteIPv4, remotePort)

	err = udpmultipath.InterceptOngoingConnection(udpConn.LocalPort, packetChan)
	if err != nil {
		log.Fatalf("Couldn't intercept the connection %v\n", err)
		os.Exit(1)
	}

	err = udpmultipath.InterceptIncomingConnection(udpConn.LocalPort)
	if err != nil {
		log.Fatalf("Couldn't re-inject incoming packets into the client: %v\n", err)
		os.Exit(1)
	}

	err = udpmultipath.Multipath(localIPv4, proxyIP, nil, packetChan)
	if err != nil {
		log.Fatalf("Couldn't make a multipath connection %v\n", err)
		os.Exit(1)
	}
}
