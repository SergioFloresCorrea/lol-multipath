package udpmultipath

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptrace"
	"sort"
	"sync"
	"time"
)

const (
	timeout         = 1 * time.Second
	badPing         = 2000 // 2 seconds
	maxConnections  = 2
	thresholdFactor = 1.4 // drop anything more than +40% above best
)

var (
	RAND, _   = randomHex(11)
	serverMap = map[string]string{
		"NA":   fmt.Sprintf("https://dynamodb.us-east-1.amazonaws.com/ping?x=%s", RAND),
		"LAN":  fmt.Sprintf("https://dynamodb.us-east-1.amazonaws.com/ping?x=%s", RAND),
		"LAS":  fmt.Sprintf("https://dynamodb.sa-east-1.amazonaws.com/ping?x=%s", RAND),
		"EUW":  fmt.Sprintf("https://dynamodb.eu-central-1.amazonaws.com/ping?x=%s", RAND),
		"OCE":  fmt.Sprintf("https://dynamodb.ap-southeast-2.amazonaws.com/ping?x=%s", RAND),
		"EUNE": fmt.Sprintf("https://dynamodb.eu-central-1.amazonaws.com/ping?x=%s", RAND),
		"RU":   fmt.Sprintf("https://dynamodb.eu-north-1.amazonaws.com/ping?x=%s", RAND),
		"TR":   fmt.Sprintf("https://dynamodb.eu-south-1.amazonaws.com/ping?x=%s", RAND),
		"JP":   fmt.Sprintf("https://dynamodb.ap-northeast-1.amazonaws.com/ping?x=%s", RAND),
		"KR":   fmt.Sprintf("https://dynamodb.ap-northeast-2.amazonaws.com/ping?x=%s", RAND),
	}
	pingMu sync.Mutex
)

func selectBestConnections(conns []*UdpConnection, pingConn []net.Conn, firstTime *bool) ([]*UdpConnection, error) {
	/*
		This function should only be used if there is a proxy. If there isn't, then it will fail
	*/
	var wg sync.WaitGroup
	results := make(chan result, len(conns))

	for index, _ := range pingConn {
		wg.Add(1)
		go func(c *UdpConnection, pingConn net.Conn) {
			defer wg.Done()
			p, err := UDPing(pingConn, timeout)
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
		if len(all) > maxConnections {
			sort.Slice(all, func(i, j int) bool {
				return all[i].ping < all[j].ping
			})
			cutoff := int64(float64(all[0].ping) * thresholdFactor)
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

	showPings(all)

	bestConnections := make([]*UdpConnection, len(all))
	for index, _ := range all {
		bestConnections[index] = all[index].conn
	}

	return bestConnections, nil
}

func GetBloat(server string) (time.Duration, error) {
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
	client := &http.Client{Transport: tr}

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
func UDPing(conn net.Conn, timeout time.Duration) (int64, error) {
	pingMu.Lock()
	defer pingMu.Unlock()

	// Drain any packets already queued in the socket
	drainBuf := make([]byte, 8)
	if err := conn.SetReadDeadline(time.Now().Add(10 * time.Millisecond)); err != nil {
		return 0, err
	}
	for {
		if _, err := conn.Read(drainBuf); err != nil {
			break
		}
	}
	// Clear the read deadline
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		return 0, err
	}

	// Do the real ping with a full timeout
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return 0, err
	}

	t0 := time.Now()
	dummy := make([]byte, 8)
	if _, err := conn.Write(dummy); err != nil {
		return 0, err
	}

	resp := make([]byte, 8)
	if _, err := conn.Read(resp); err != nil {
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

func randomHex(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func getClosest(obj []result, maxPing int64) int {
	/*
		assumes the results are ordered in ascending order by ping
	*/
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

	// At this point, hi < lo, and:
	//  - hi is the index of the largest element < maxPing (or -1 if none)
	//  - lo is the index of the smallest element > maxPing (or n if none)
	// Compare those two candidates:
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

func absInt64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

func showPings(objs []result) {
	var showObjs []result
	show := ""
	if len(objs) > maxConnections {
		showObjs = objs[:maxConnections]
	} else {
		showObjs = objs
	}

	for _, obj := range showObjs {
		show += fmt.Sprintf("Expected ping for connection %s->%s: %d (ms)\n", obj.conn.conn.LocalAddr(), obj.conn.conn.RemoteAddr(), obj.ping)
	}
	log.Printf("%v", show)
}
