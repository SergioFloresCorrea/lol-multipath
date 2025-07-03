package udpmultipath

import (
	"fmt"
	"time"
)

const (
	badPing = 2000 // Default value for a really bad ping in ms.
)

type Config struct {
	ServerMap       map[string]string // maps league servers to DynamoDB endpoints for ping calculation
	Server          string
	Rand            string        // random hex number for ping HTTP queries
	ThresholdFactor float64       // drop connections whose ping > factor×lowest ping
	UpdateInterval  time.Duration // how often to refresh ping metrics
	Timeout         time.Duration // how long to wait for a ping response
	ProbeInterval   time.Duration // how long to wait for probing down connections
	CleanupInterval time.Duration // how long to wait before cleaning the packet cache involved in the deduplicating package process
	MaxConnections  int           // maximum number of multipath connections
	Dynamic         bool          // enable periodic proxy reselection

}

func (cfg *Config) GenerateServerMap() {
	if cfg.ServerMap == nil {
		cfg.ServerMap = map[string]string{
			"NA":   fmt.Sprintf("https://dynamodb.us-east-2.amazonaws.com/ping?x=%s", cfg.Rand),
			"LAN":  fmt.Sprintf("https://dynamodb.us-east-1.amazonaws.com/ping?x=%s", cfg.Rand),
			"LAS":  fmt.Sprintf("https://dynamodb.sa-east-1.amazonaws.com/ping?x=%s", cfg.Rand),
			"EUW":  fmt.Sprintf("https://dynamodb.eu-central-1.amazonaws.com/ping?x=%s", cfg.Rand),
			"OCE":  fmt.Sprintf("https://dynamodb.ap-southeast-2.amazonaws.com/ping?x=%s", cfg.Rand),
			"EUNE": fmt.Sprintf("https://dynamodb.eu-central-1.amazonaws.com/ping?x=%s", cfg.Rand),
			"RU":   fmt.Sprintf("https://dynamodb.eu-north-1.amazonaws.com/ping?x=%s", cfg.Rand),
			"TR":   fmt.Sprintf("https://dynamodb.eu-south-1.amazonaws.com/ping?x=%s", cfg.Rand),
			"JP":   fmt.Sprintf("https://dynamodb.ap-northeast-1.amazonaws.com/ping?x=%s", cfg.Rand),
			"KR":   fmt.Sprintf("https://dynamodb.ap-northeast-2.amazonaws.com/ping?x=%s", cfg.Rand),
		}
	}
}
