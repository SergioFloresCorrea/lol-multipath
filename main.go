package main

import (
	"log"
	"os"

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
}
