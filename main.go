package main

import (
	"log"
	"os"

	"github.com/SergioFloresCorrea/bondcat-reduceping/connection"
)

func main() {
	filePath := "connection/ips.txt"
	protocol := "TCP"
	remoteIPs, localIPs, err := connection.ResolveRiotIP(filePath, protocol)
	if err != nil {
		log.Fatalf("Failed: %v", err)
		os.Exit(1)
	}

	log.Printf("Local IPs: %v\n Remote IPs: %v\n", localIPs, remoteIPs)
}
