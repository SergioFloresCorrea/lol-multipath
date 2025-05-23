package connection

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type ConnectionUDP struct {
	LocalAddress string
	LocalPort    string
}

func GetUDPConnection() (ConnectionUDP, error) {
	commandString := fmt.Sprintf(`Get-NetUDPEndpoint | Where-Object { $_.OwningProcess -eq (Get-Process -Name "%s").Id }`, leagueProcessName)
	cmd := exec.Command("powershell.exe", "-Command", commandString)
	cmd.Stderr = os.Stderr

	output, err := cmd.Output()

	if err != nil {
		return ConnectionUDP{}, fmt.Errorf("error in running the powershell command %v", err)
	}

	if len(output) > 0 {
		str := string(output)
		lines := strings.Split(str, "\n")
		for _, line := range lines {
			if strings.Contains(line, "Get-Process") {
				return ConnectionUDP{}, fmt.Errorf("couldn't find the process %v", leagueProcessName)
			}
			if line == "" || strings.Contains(line, "LocalAddress") || strings.Contains(line, "-") {
				continue
			}

			fields := strings.Fields(line)

			if len(fields) >= 2 {
				connection := ConnectionUDP{LocalAddress: fields[0], LocalPort: fields[1]}
				return connection, nil
			}

		}
	}

	return ConnectionUDP{}, fmt.Errorf("couldn't find any connection")
}
