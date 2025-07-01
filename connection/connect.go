package connection

import (
	"context"
	"time"
)

// Waits until `leagueNameProcess` starts (i.e after starting a game) and sends
// the LocalIP, LocalPort and any error that may arise during the process.
// It checks every `interval` for the game, so it may not find the process inmediately.
func WaitForLeagueAndResolve(ctx context.Context, interval time.Duration) (UDPResult, error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return UDPResult{}, ctx.Err()

		case <-ticker.C:
			conn, err := GetUDPConnection()
			if err == nil {
				return UDPResult{LocalIP: conn.LocalAddress, LocalPort: conn.LocalPort}, err
			}
		}
	}
}
