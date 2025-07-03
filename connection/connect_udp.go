package connection

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Uses a powershell command to find the port and the local address `leagueProcessName` uses to listen
// for UDP traffic.
func GetUDPConnection() (ConnectionUDP, error) {
	commandString := fmt.Sprintf(`Get-NetUDPEndpoint | Where-Object { $_.OwningProcess -eq (Get-Process -Name "%s").Id } | Select-Object LocalAddress,LocalPort | ConvertTo-Json -Depth 2`, leagueProcessName)
	cmd := exec.Command("powershell.exe", "-Command", commandString)
	cmd.Stderr = os.Stderr

	output, err := cmd.Output()

	if err != nil {
		return ConnectionUDP{}, fmt.Errorf("error in running the powershell command %w", err)
	}

	if len(output) > 0 {
		var endpoint ConnectionUDP
		if err := json.Unmarshal(output, &endpoint); err != nil {
			return ConnectionUDP{}, fmt.Errorf("couldn't find the process %v", leagueProcessName)
		}
		return endpoint, nil
	}

	return ConnectionUDP{}, fmt.Errorf("couldn't find any connection")
}

func CheckIfLeagueIsActive() bool {
	commandString := fmt.Sprintf(`Get-Process -Name "%s" -ErrorAction SilentlyContinue`, leagueProcessName)
	cmd := exec.Command("powershell.exe", "-Command", commandString)

	output, _ := cmd.Output()

	if strings.TrimSpace(string(output)) != "" {
		return true
	}
	return false
}
