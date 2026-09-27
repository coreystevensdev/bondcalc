package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coreystevensdev/bondcalc/internal/api"
	"github.com/gin-gonic/gin"
)

func limitedRouter(limit int, window time.Duration) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(api.RateLimit(limit, window))
	r.GET("/probe", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func probe(r *gin.Engine, ip string) int {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/probe", nil)
	req.RemoteAddr = ip + ":1234"
	r.ServeHTTP(w, req)
	return w.Code
}

func TestRateLimitAllowsUpToTheLimit(t *testing.T) {
	r := limitedRouter(3, time.Minute)

	for i := 1; i <= 3; i++ {
		if code := probe(r, "198.51.100.1"); code != http.StatusOK {
			t.Fatalf("request %d of 3 should pass, got %d", i, code)
		}
	}
}

func TestRateLimitRejectsPastTheLimit(t *testing.T) {
	r := limitedRouter(3, time.Minute)

	for i := 0; i < 3; i++ {
		probe(r, "198.51.100.2")
	}
	if code := probe(r, "198.51.100.2"); code != http.StatusTooManyRequests {
		t.Fatalf("4th request should be 429, got %d", code)
	}
}

// Without this, one visitor hammering the demo would lock out everyone else.
func TestRateLimitIsPerClient(t *testing.T) {
	r := limitedRouter(2, time.Minute)

	probe(r, "198.51.100.3")
	probe(r, "198.51.100.3")
	if code := probe(r, "198.51.100.3"); code != http.StatusTooManyRequests {
		t.Fatalf("first client should be limited, got %d", code)
	}
	if code := probe(r, "198.51.100.4"); code != http.StatusOK {
		t.Fatalf("second client should be unaffected, got %d", code)
	}
}

// Sliding, not fixed: once the oldest hit ages out the caller gets that slot
// back, rather than every caller resetting together on a wall-clock boundary.
func TestRateLimitSlotsComeBackAsTheyAge(t *testing.T) {
	r := limitedRouter(2, 80*time.Millisecond)

	probe(r, "198.51.100.5")
	time.Sleep(50 * time.Millisecond)
	probe(r, "198.51.100.5")
	if code := probe(r, "198.51.100.5"); code != http.StatusTooManyRequests {
		t.Fatalf("both slots used, expected 429, got %d", code)
	}

	// Only the first hit has aged out, so exactly one slot should free up.
	time.Sleep(40 * time.Millisecond)
	if code := probe(r, "198.51.100.5"); code != http.StatusOK {
		t.Fatalf("oldest hit aged out, expected 200, got %d", code)
	}
	if code := probe(r, "198.51.100.5"); code != http.StatusTooManyRequests {
		t.Fatalf("second hit has not aged out yet, expected 429, got %d", code)
	}
}

// gin trusts X-Forwarded-For from any source unless trusted proxies are set, so
// a limiter keyed on ClientIP() would be bypassable by rotating one header.
func TestRateLimitIgnoresForwardedForHeader(t *testing.T) {
	r := limitedRouter(2, time.Minute)

	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/probe", nil)
		req.RemoteAddr = "198.51.100.9:1234"
		r.ServeHTTP(w, req)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/probe", nil)
	req.RemoteAddr = "198.51.100.9:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.77")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("spoofed X-Forwarded-For bypassed the limit, got %d", w.Code)
	}
}
