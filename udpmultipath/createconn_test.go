package udpmultipath

import (
	"net"
	"testing"
)

func TestCreateDialers(t *testing.T) {
	localIPs := []net.IP{
		net.ParseIP("127.0.0.1"),
		net.ParseIP("127.0.0.2"),
	}
	ds, err := createDialers(localIPs)
	if err != nil {
		t.Fatalf("createDialers: unexpected error: %v", err)
	}
	if len(ds) != len(localIPs) {
		t.Fatalf("len(dialers) = %d; want %d", len(ds), len(localIPs))
	}
	for i, d := range ds {
		udpAddr, ok := d.LocalAddr.(*net.UDPAddr)
		if !ok {
			t.Errorf("dialer %d: LocalAddr is %T, want *net.UDPAddr", i, d.LocalAddr)
			continue
		}
		if !udpAddr.IP.Equal(localIPs[i]) {
			t.Errorf("dialer %d: IP = %v; want %v", i, udpAddr.IP, localIPs[i])
		}
		if udpAddr.Port != 0 {
			t.Errorf("dialer %d: Port = %d; want 0", i, udpAddr.Port)
		}
	}
}

func TestCreateConnections_Success(t *testing.T) {
	localIPs := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.1")}
	dialers, _ := createDialers(localIPs)

	targets := []string{"127.0.0.1:40000", "127.0.0.1:40001"}
	pings := []string{"127.0.0.1:50000", "127.0.0.1:50001"}

	connPort, err := createConnections(dialers, targets, pings)
	if err != nil {
		t.Fatalf("createConnections returned error: %v", err)
	}
	// 2 dialers × 2 targets = 4 each
	want := len(dialers) * len(targets)
	if got := len(connPort.UDPConns); got != want {
		t.Errorf("UDPConns length = %d; want %d", got, want)
	}
	if got := len(connPort.PingConns); got != want {
		t.Errorf("PingConns length = %d; want %d", got, want)
	}

	// one-to-one: UDPConns[i].RemoteAddr() ↔ targets[i%2], PingConns[i] ↔ pings[i%2]
	for i := range want {
		uRem := connPort.UDPConns[i].conn.RemoteAddr().String()
		pRem := connPort.PingConns[i].conn.RemoteAddr().String()
		if wantU := targets[i%len(targets)]; uRem != wantU {
			t.Errorf("UDPConns[%d].RemoteAddr = %q; want %q", i, uRem, wantU)
		}
		if wantP := pings[i%len(pings)]; pRem != wantP {
			t.Errorf("PingConns[%d].RemoteAddr = %q; want %q", i, pRem, wantP)
		}
	}

	// cleanup
	closeConnections(connPort.UDPConns)
	closeConnections(connPort.PingConns)
}
