package udpmultipath

import (
	"reflect"
	"testing"
)

// Helper to pull out just the ping values from a []result.
func pings(rs []result) []int64 {
	out := make([]int64, len(rs))
	for i, r := range rs {
		out[i] = r.ping
	}
	return out
}

// Helper to generate a slice of result with just ping
func makeResults(pings []int64) []result {
	rs := make([]result, len(pings))
	for i, p := range pings {
		rs[i] = result{ping: p}
	}
	return rs
}

func TestSelectAndCloseConnections(t *testing.T) {
	tests := []struct {
		name               string
		firstTime          bool
		cfg                Config
		inputPings         []int64
		wantOutPings       []int64
		wantClosedPings    []int64
		wantFirstTimeAfter bool
	}{
		{
			name:               "not first time — just sort",
			firstTime:          false,
			cfg:                Config{ThresholdFactor: 1.4, MaxConnections: 2},
			inputPings:         []int64{120, 100, 180, 150},
			wantOutPings:       []int64{100, 120, 150, 180},
			wantClosedPings:    []int64{},
			wantFirstTimeAfter: false,
		},
		{
			name:               "first time — trims above threshold",
			firstTime:          true,
			cfg:                Config{ThresholdFactor: 1.4, MaxConnections: 2},
			inputPings:         []int64{120, 100, 180, 150},
			wantOutPings:       []int64{100, 120},
			wantClosedPings:    []int64{150, 180},
			wantFirstTimeAfter: false,
		},
		{
			name:               "first time but len ≤ MaxConnections",
			firstTime:          true,
			cfg:                Config{ThresholdFactor: 2.0, MaxConnections: 5},
			inputPings:         []int64{50, 30, 20},
			wantOutPings:       []int64{20, 30, 50},
			wantClosedPings:    []int64{},
			wantFirstTimeAfter: false,
		},
		{
			name:               "low threshold factor — only 1 kept",
			firstTime:          true,
			cfg:                Config{ThresholdFactor: 1.1, MaxConnections: 3},
			inputPings:         []int64{10, 20, 30, 40},
			wantOutPings:       []int64{10},
			wantClosedPings:    []int64{20, 30, 40},
			wantFirstTimeAfter: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := makeResults(tc.inputPings)
			first := tc.firstTime

			out, closed := tc.cfg.selectAndCloseConnections(in, &first)

			// 1. firstTime flag flipped?
			if first != tc.wantFirstTimeAfter {
				t.Errorf("firstTime: %v, want %v", first, tc.wantFirstTimeAfter)
			}

			// 2. sorted/trimmed output
			gotOut := pings(out)
			if !reflect.DeepEqual(gotOut, tc.wantOutPings) {
				t.Errorf("out pings = %v, want %v", gotOut, tc.wantOutPings)
			}

			// 3. which got closed?
			gotClosed := pings(closed)
			if !reflect.DeepEqual(gotClosed, tc.wantClosedPings) {
				t.Errorf("closed pings = %v, want %v", gotClosed, tc.wantClosedPings)
			}
		})
	}
}

func TestGetClosest(t *testing.T) {
	tests := []struct {
		name       string
		maxPing    int64
		inputPings []int64
		wantIndex  int
	}{
		{
			name:       "max ping greater not in the slice",
			maxPing:    150,
			inputPings: []int64{100, 120, 140, 160, 180},
			wantIndex:  2,
		},
		{
			name:       "max ping in the slice",
			maxPing:    140,
			inputPings: []int64{100, 120, 140, 160, 180},
			wantIndex:  2,
		},
		{
			name:       "max ping at the start",
			maxPing:    100,
			inputPings: []int64{100, 135, 280, 300},
			wantIndex:  0,
		},
		{
			name:       "max ping greater than the end",
			maxPing:    200,
			inputPings: []int64{120, 150, 190, 200},
			wantIndex:  3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := makeResults(tc.inputPings)
			maxPing := tc.maxPing

			outIdx := getClosest(in, maxPing)
			if outIdx != tc.wantIndex {
				t.Errorf("input = %v, max ping = %d\n expected index: %d, out index: %d", tc.inputPings, maxPing, tc.wantIndex, outIdx)
			}
		})
	}
}
