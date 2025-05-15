package main

import (
	"time"

	"github.com/SergioFloresCorrea/bondcat-reduceping/connection"
)

func main() {
	done := make(chan struct{})
	go connection.WaitForLeagueAndResolve("UDP", 2*time.Second, done)

	// Optionally block main from exiting
	<-done
}
