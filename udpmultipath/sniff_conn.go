package udpmultipath

import (
	"bufio"
	"io"
	"log"
	"os"
	"os/exec"
)

func SniffConnection(port string, packetChan chan []byte) error {
	var cmd *exec.Cmd

	cmd = exec.Command("tools\\gor.exe", "--input-udp", ":"+port, "--output-stdout")

	cmd.Stderr = os.Stderr
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	go func() {
		reader := bufio.NewReader(stdoutPipe)
		buffer := make([]byte, 4096) // Adjust buffer size as needed

		for {
			n, err := reader.Read(buffer)
			if err != nil {
				if err == io.EOF {
					log.Println("End of stdout stream")
					break
				}
				log.Fatalf("error reading stdout: %v", err)
			}

			if n > 0 {
				packet := make([]byte, n)
				copy(packet, buffer[:n])
				// log.Println("Sending packet to packetChan")
				packetChan <- packet
			}
		}
	}()

	return nil
}
