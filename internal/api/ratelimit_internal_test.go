package api

import (
	"testing"
	"time"
)

// One entry per source address, kept for the life of the process, is a memory
// leak a caller can drive by rotating addresses. The sweep is what bounds it.
func TestIdleKeysAreEvicted(t *testing.T) {
	l := &limiter{
		hits:   make(map[string][]time.Time),
		limit:  5,
		window: time.Millisecond,
	}

	for i := 0; i < sweepEvery-1; i++ {
		l.allow(uniqueAddr(i))
	}

	// 256 map inserts take microseconds, so without this the keys are all still
	// inside the window when the sweep runs and nothing is eligible.
	time.Sleep(2 * time.Millisecond)
	l.allow("10.255.255.255") // the call that trips the sweep

	// Everything above has aged past the 1ms window, so a bounded map should be
	// far smaller than the number of callers seen.
	if len(l.hits) > sweepEvery/2 {
		t.Fatalf("expected idle keys to be swept, map still holds %d of %d", len(l.hits), sweepEvery)
	}
}

func uniqueAddr(i int) string {
	return "10.0." + itoa(i/256) + "." + itoa(i%256)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
