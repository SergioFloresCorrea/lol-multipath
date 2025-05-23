package udpmultipath

import (
	"net"
	"time"
)

const DummyPort = 10001

func DummyTraffic(port int) {
	go dummyListener(port)
	for {
		dummyDialer(port)
		time.Sleep(1 * time.Second)
	}
}

func dummyListener(port int) error {
	listener, err := net.ListenUDP("udp", &net.UDPAddr{IP: []byte{0, 0, 0, 0}, Port: port})
	if err != nil {
		return err
	}
	defer listener.Close()

	buf := make([]byte, 1024)
	for {
		_, _, err = listener.ReadFromUDP(buf)
		if err != nil {
			return err
		}
	}
}

func dummyDialer(port int) error {
	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: []byte{127, 0, 0, 1}, Port: port})
	if err != nil {
		return err
	}
	defer conn.Close()

	conn.Write([]byte("hello"))
	return nil
}
