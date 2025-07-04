package connection

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Uses a powershell command to find the port and the local address `leagueProcessName` uses to listen
// for UDP traffic.
func GetUDPConnection(interval time.Duration) (ConnectionUDP, error) {
	commandString := fmt.Sprintf(`Get-NetUDPEndpoint | Where-Object { $_.OwningProcess -eq (Get-Process -Name "%s").Id } | Select-Object LocalAddress,LocalPort | ConvertTo-Json -Depth 2`, leagueProcessName)
	ctx, cancel := context.WithTimeout(context.Background(), interval)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell.exe", "-Command", commandString) // #nosec G204
	cmd.Stderr = os.Stderr

	output, err := cmd.Output()

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return ConnectionUDP{}, fmt.Errorf("timed out after %s", interval)
		}
		return ConnectionUDP{}, fmt.Errorf("powershell failed: %v\noutput: %s", err, output)
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
	cmd := exec.Command("powershell.exe", "-Command", commandString) // #nosec G204

	output, _ := cmd.Output()

	if strings.TrimSpace(string(output)) != "" {
		return true
	}
	return false
}
