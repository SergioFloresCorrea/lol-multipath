package main

import (
	"log"
	"net"
	"os"
	"time"

	"github.com/SergioFloresCorrea/bondcat-reduceping/connection"
	"github.com/SergioFloresCorrea/bondcat-reduceping/udpmultipath"
)

const hostname = "la1.api.riotgames.com"

func main() {
	var udpConn connection.UDPResult

	done := make(chan struct{})
	tcpResultChan := make(chan connection.TCPResult, 1)
	udpResultChan := make(chan connection.UDPResult, 1)

	go connection.WaitForLeagueAndResolve("UDP", 10*time.Second, done, tcpResultChan, udpResultChan)

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
	remoteIPv4, remoteIPv6, err := udpmultipath.GetRemoteAddresses(hostname)
	if err != nil {
		log.Fatalf("%v\n", err)
		os.Exit(1)
	}

	log.Printf("Local Interface IPv4 addresses: %v", localIPv4)
	log.Printf("Local Interface IPv6 addresses: %v", localIPv6)

	log.Printf("Remote %v IPv4 addresses: %v", hostname, remoteIPv4)
	log.Printf("Remote %v IPv6 addresses: %v", hostname, remoteIPv6)

	proxyIP := net.ParseIP(udpmultipath.ListenIPString) // or the IP where dummy_proxy is listening
	go udpmultipath.ProxyServer()

	err = udpmultipath.SniffConnection(udpConn.LocalPort, packetChan)
	if err != nil {
		log.Fatalf("Couldn't sniff the connection %v\n", err)
		os.Exit(1)
	}

	err = udpmultipath.Multipath(localIPv4, proxyIP, nil, packetChan)
	if err != nil {
		log.Fatalf("Couldn't make a multipath connection %v\n", err)
		os.Exit(1)
	}
}
