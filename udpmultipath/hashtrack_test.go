package udpmultipath

import (
	"reflect"
	"sort"
	"testing"
	"time"
)

func getKeys(tracker *SeenHashTracker) []uint64 {
	keys := make([]uint64, 0, len(tracker.SeenHash))
	for k := range tracker.SeenHash {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

func TestTracker(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name    string
		entries map[uint64]time.Time
		want    []uint64
	}{
		{
			name: "all fresh",
			entries: map[uint64]time.Time{
				1: now,
				2: now.Add(-10 * time.Millisecond),
			},
			want: []uint64{1, 2},
		},
		{
			name: "some expired",
			entries: map[uint64]time.Time{
				3: now.Add(-200 * time.Millisecond),
				4: now.Add(-10 * time.Millisecond),
			},
			want: []uint64{4},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{CleanupInterval: 50 * time.Millisecond}
			tracker := cfg.newTracker()
			tracker.SeenHash = tc.entries

			tracker.cleanupHash()
			got := getKeys(tracker)

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("after cleanup got %v; want %v", got, tc.want)
			}
		})
	}
}

func TestIsHashDuplicate(t *testing.T) {
	cfg := Config{CleanupInterval: time.Minute}
	tracker := cfg.newTracker()

	if got := tracker.isHashDuplicate(42); got {
		t.Errorf("expected isHashDuplicate(42) == false on first call, got %v", got)
	}

	if got := tracker.isHashDuplicate(42); !got { // duplicate
		t.Errorf("expected isHashDuplicate(42) == true on second call, got %v", got)
	}

	if len(tracker.SeenHash) != 1 {
		t.Fatalf("expected exactly 1 key, got %d", len(tracker.SeenHash))
	}
}
