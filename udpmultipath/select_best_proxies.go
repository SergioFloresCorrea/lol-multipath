package udpmultipath

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptrace"
	"sort"
	"sync"
	"time"
)

var (
	pingMu sync.Mutex
)

// Pings every connection and returns them in ascending order. Depending on `firstTime` it trims them depending
// on whether their ping exceeds 40% from the least ping.
func (cfg *Config) selectBestConnections(conns []*UdpConnection, pingConn []*UdpConnection, firstTime *bool) ([]*UdpConnection, error) {
	var wg sync.WaitGroup
	results := make(chan result, len(conns))

	for index, _ := range pingConn {
		wg.Add(1)
		go func(c *UdpConnection, pingConn *UdpConnection) {
			defer wg.Done()
			p, err := cfg.udping(pingConn)
			if err != nil {
				p = badPing
			}
			results <- result{c, pingConn, p}
		}(conns[index], pingConn[index])
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var all []result
	var toBeClosed []result
	for r := range results {
		all = append(all, r)
	}

	if *firstTime {
		// grab smallest pings considering the threshold
		if len(all) > cfg.MaxConnections {
			sort.Slice(all, func(i, j int) bool {
				return all[i].ping < all[j].ping
			})
			cutoff := int64(float64(all[0].ping) * cfg.ThresholdFactor)
			cutoffIndex := getClosest(all, cutoff)
			toBeClosed = all[cutoffIndex:]
			all = all[:cutoffIndex]
		}
		*firstTime = false

		// close unneded connections
		toBeClosedConnections := make([]*UdpConnection, len(toBeClosed))
		for index, _ := range toBeClosed {
			toBeClosedConnections[index] = toBeClosed[index].conn
		}
		closeConnections(toBeClosedConnections)
	}

	cfg.showPings(all)

	bestConnections := make([]*UdpConnection, len(all))
	for index, _ := range all {
		bestConnections[index] = all[index].conn
	}

	return bestConnections, nil
}

// Sends a HTTP request to the League `server` (i.e LAN, LAS, NA, etc) and calculates the time it needs for
// the DNS resolution + TCP handshake + (things that do are not how long the packet sent takes to make a roundtrip)
// to occur. Idea taken from https://pingtestlive.com/league-of-legends
func GetBloat(serverMap map[string]string, server string) (time.Duration, error) {
	t0 := time.Now()
	url, ok := serverMap[server]
	if !ok {
		return 0, fmt.Errorf("unknown server %q. Please use flag -servers to see which are available", server)
	}

	var (
		tWriteDone time.Time
		tFirstByte time.Time
	)

	// set up a trace to record exactly when the request is sent
	trace := &httptrace.ClientTrace{
		WroteRequest: func(info httptrace.WroteRequestInfo) {
			tWriteDone = time.Now()
		},
		GotFirstResponseByte: func() {
			tFirstByte = time.Now()
		},
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.DisableKeepAlives = true
	client := &http.Client{
		Transport: tr,
		Timeout:   2 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return 0, fmt.Errorf("reading body: %w", err)
	}

	if tWriteDone.IsZero() || tFirstByte.IsZero() {
		return 0, fmt.Errorf("trace events missing")
	}

	t1 := time.Now()
	latency := tFirstByte.Sub(tWriteDone)
	totalTime := t1.Sub(t0)
	bloat := totalTime - latency

	return bloat, nil
}

// UDPing sends an 8-byte dummy packet over conn, waits for the 8-byte
// “bloat” reply, and returns the true UDP RTT (total – bloat) in ms.
func (cfg *Config) udping(conn *UdpConnection) (int64, error) {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	// Drain any packets already queued in the socket
	drainBuf := make([]byte, 8)
	if err := conn.conn.SetReadDeadline(time.Now().Add(10 * time.Millisecond)); err != nil {
		return 0, err
	}
	for {
		if _, err := conn.conn.Read(drainBuf); err != nil {
			break
		}
	}
	// Clear the read deadline
	if err := conn.conn.SetReadDeadline(time.Time{}); err != nil {
		return 0, err
	}

	// Do the real ping with a full timeout
	if err := conn.conn.SetDeadline(time.Now().Add(cfg.Timeout)); err != nil {
		return 0, err
	}

	t0 := time.Now()
	dummy := make([]byte, 8)
	if _, err := conn.conn.Write(dummy); err != nil {
		return 0, err
	}

	resp := make([]byte, 8)
	if _, err := conn.conn.Read(resp); err != nil {
		return 0, err
	}
	total := time.Since(t0)

	// Decode the “bloat” (HTTP proxy delay) in ms
	bloatMs := binary.BigEndian.Uint64(resp)
	bloat := time.Duration(bloatMs) * time.Millisecond

	// True UDP RTT
	pingDur := total - bloat
	log.Printf("Bloat: %v  Total RTT: %v  UDP-only ping: %v", bloat, total, pingDur)

	return pingDur.Milliseconds(), nil
}

// Assumes the results are ordered in ascending order by ping.
// Returns the index of the closest but not exceeding element in `obj` ping
// with respect to maxPing
func getClosest(obj []result, maxPing int64) int {
	n := len(obj)
	if n == 0 {
		return -1
	}

	lo, hi := 0, n-1
	for lo <= hi {
		mid := lo + (hi-lo)/2
		switch {
		case obj[mid].ping == maxPing:
			return mid
		case obj[mid].ping < maxPing:
			lo = mid + 1
		default:
			hi = mid - 1
		}
	}

	bestIdx := hi
	bestDiff := absInt64(maxPing - obj[hi].ping)
	if lo < n {
		if d := absInt64(obj[lo].ping - maxPing); lo == -1 || d < bestDiff {
			bestDiff = d
			bestIdx = lo
		}
	}
	return bestIdx
}

// Brute force and totally ashaming of getting the absolute value of an int64.
func absInt64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

// Logs in console the current ping for the best `maxConnections` connections.
func (cfg *Config) showPings(objs []result) {
	var showObjs []result
	show := ""
	if len(objs) > cfg.MaxConnections {
		showObjs = objs[:cfg.MaxConnections]
	} else {
		showObjs = objs
	}

	for _, obj := range showObjs {
		show += fmt.Sprintf("Expected ping for connection %s->%s: %d (ms)\n", obj.conn.conn.LocalAddr(), obj.conn.conn.RemoteAddr(), obj.ping)
	}
	log.Printf("%v", show)
}
