package connection

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

const leagueProcessName = "League of Legends"

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

	leaguePid, err := GetRiotPID()
	if err != nil {
		return nil, nil, err
	}

	remoteIPs, localIPs := matchPIDConnection(connections, leaguePid)

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

func matchPIDConnection(connections []Connection, pid string) ([]string, []string) {
	remoteIPs := make([]string, 0)
	localIPs := make([]string, 0)
	for _, conn := range connections {
		if conn.PID == pid {
			remoteIPs = append(remoteIPs, conn.RemoteAddress)
			localIPs = append(localIPs, conn.LocalAddress)
		}
	}

	return remoteIPs, localIPs
}

func GetRiotPID() (string, error) {
	cmd := exec.Command("tasklist.exe", "/FI", fmt.Sprintf("IMAGENAME eq %s", leagueProcessName))
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("unable to run the command tasklist.exe: %v", err)
	}
	outputString := string(output)
	if strings.Contains(outputString, "INFORMACION:") {
		return "", fmt.Errorf("unable to find %s", leagueProcessName)
	}
	lines := strings.Split(outputString, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "Nombre de Imagen") || strings.Contains(line, "=") {
			continue
		}

		fields := strings.Fields(line)
		if strings.Contains(line, leagueProcessName) {
			for _, f := range fields {
				if _, err := strconv.Atoi(f); err == nil {
					return f, nil
				}
			}
		}

	}

	return "", fmt.Errorf("unable to get the PID of %s", leagueProcessName)
}
