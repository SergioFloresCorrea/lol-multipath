package connection

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type Connection struct {
	Protocol      string
	LocalAddress  string
	RemoteAddress string
	State         string
	PID           string
}

type PIDs struct {
	Protocol      string
	PID           string
	NumberOfPorts string
}

func ResolveRiotIP(filePath, protocol string) ([]string, []string, error) {
	err := writeNetstat(filePath, protocol)
	if err != nil {
		return nil, nil, err
	}

	connections, err := readNetstatFile(filePath, protocol)
	if err != nil {
		return nil, nil, err
	}

	pids, err := orderPIDbyPorts(protocol)
	if err != nil {
		return nil, nil, err
	}

	filteredPid, err := filterRiotPID(pids)
	if err != nil {
		return nil, nil, err
	}

	remoteIPs, localIPs := matchPIDConnection(connections, filteredPid)

	totalPorts, err := strconv.Atoi(filteredPid.NumberOfPorts)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't convert string to integer: %v", err)
	}

	if len(remoteIPs) != totalPorts {
		return nil, nil, fmt.Errorf("couldn't resolve all remote addresses, missing %d", totalPorts-len(remoteIPs))
	}

	return remoteIPs, localIPs, nil
}

func writeNetstat(filePath, protocol string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("couldn't create the file %s", filePath)
	}
	defer file.Close()

	// Run Windows netstat from WSL (path to netstat.exe) and filter for TCP
	cmd := exec.Command("/mnt/c/Windows/System32/netstat.exe", "-ano", "-p", protocol)
	cmd.Stdout = file

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("netstat failed: %v", err)
	}

	return nil
}

func readNetstatFile(filePath, protocol string) ([]Connection, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("couldn't open the file %s", filePath)
	}
	defer file.Close()

	connections := make([]Connection, 0)

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.Contains(line, "Proto") || strings.Contains(line, "Conexiones") {
			continue
		}

		fields := strings.Fields(line)

		if len(fields) < 5 {
			continue // Skip malformed lines
		}

		if fields[0] != protocol { // The first field must be equal to the protocol (e.g TCP, UDP)
			continue
		}

		connection := Connection{
			Protocol:      fields[0],
			LocalAddress:  fields[1],
			RemoteAddress: fields[2],
			State:         fields[3],
			PID:           fields[4],
		}

		connections = append(connections, connection)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error in reading the file: %v", err)
	}

	return connections, nil
}

func orderPIDbyPorts(protocol string) ([]PIDs, error) {
	file, err := os.CreateTemp("/tmp", "pids-*.txt")
	if err != nil {
		return nil, fmt.Errorf("couldn't create a temporary file")
	}
	defer os.Remove(file.Name())
	defer file.Close()

	cmd := exec.Command("/mnt/c/Windows/System32/netstat.exe", "-c")
	cmd.Stdout = file

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("couldn't run the netstat.exe command: %v", err)
	}
	file.Seek(0, 0)

	pids := make([]PIDs, 0)

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.Contains(line, "Proto") || strings.Contains(line, "Consumo") {
			continue
		}

		fields := strings.Fields(line)

		if fields[0] != protocol { // The first field must be equal to the protocol (e.g TCP, UDP)
			continue
		}

		pid := PIDs{
			Protocol:      fields[0],
			PID:           fields[1],
			NumberOfPorts: fields[2],
		}

		pids = append(pids, pid)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error in reading the file: %v", err)
	}

	return pids, nil

}

func matchPIDConnection(connections []Connection, pid PIDs) ([]string, []string) {
	remoteIPs := make([]string, 0)
	localIPs := make([]string, 0)
	for _, conn := range connections {
		if conn.PID == pid.PID {
			remoteIPs = append(remoteIPs, conn.RemoteAddress)
			localIPs = append(localIPs, conn.LocalAddress)
		}
	}

	return remoteIPs, localIPs
}

func filterRiotPID(pids []PIDs) (PIDs, error) {
	for _, pid := range pids {
		PID := pid.PID
		if PID == "0" {
			continue
		}
		cmd := exec.Command("tasklist.exe", "/FI", fmt.Sprintf("PID eq %s", PID))
		output, err := cmd.Output()
		if err != nil {
			return PIDs{}, fmt.Errorf("failed: couldn't execute the tasklist command: %v\nOutput:\n%s", err, string(output))
		}
		stringOutput := string(output)
		if strings.Contains(stringOutput, "League of Legends.exe") {
			return pid, nil
		}
	}

	return PIDs{NumberOfPorts: "0"}, fmt.Errorf("couldn't locate the league of legends.exe process")
}
