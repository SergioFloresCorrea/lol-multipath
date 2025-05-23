package main

import (
	"log"
	"net"
	"os"
	"strconv"

	"github.com/SergioFloresCorrea/bondcat-reduceping/udpmultipath"
)

const hostname = "la1.api.riotgames.com"

func main() {
	/*
		done := make(chan struct{})
		go connection.WaitForLeagueAndResolve("UDP", 2*time.Second, done)

		// Optionally block main from exiting
		<-done
	*/
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

	log.Printf("Local IPv4 addresses: %v", localIPv4)
	log.Printf("Local IPv6 addresses: %v", localIPv6)

	log.Printf("Remote IPv4 addresses: %v", remoteIPv4)
	log.Printf("Remote IPv6 addresses: %v", remoteIPv6)

	proxyIP := net.ParseIP(udpmultipath.ListenIPString) // or the IP where dummy_proxy is listening
	go udpmultipath.ProxyServer()

	go udpmultipath.DummyTraffic(udpmultipath.DummyPort)
	err = udpmultipath.SniffConnection(strconv.Itoa(udpmultipath.DummyPort), packetChan)
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
