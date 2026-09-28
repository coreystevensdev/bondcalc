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
	if err := r.SetTrustedProxies(api.TrustedProxies); err != nil {
		panic(err)
	}
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

// Once Caddy fronts this, every request arrives from loopback and the real
// caller is only in X-Forwarded-For. Without trusting that header from the proxy,
// the whole internet would share one bucket.
func TestRateLimitTrustsForwardedForFromLoopback(t *testing.T) {
	r := limitedRouter(2, time.Minute)

	hit := func(fwd string) int {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/probe", nil)
		req.RemoteAddr = "127.0.0.1:55555" // as the reverse proxy would appear
		req.Header.Set("X-Forwarded-For", fwd)
		r.ServeHTTP(w, req)
		return w.Code
	}

	hit("203.0.113.10")
	hit("203.0.113.10")
	if code := hit("203.0.113.10"); code != http.StatusTooManyRequests {
		t.Fatalf("third request from the same forwarded client should be 429, got %d", code)
	}
	// A different real client behind the same proxy must keep its own budget.
	if code := hit("203.0.113.11"); code != http.StatusOK {
		t.Fatalf("a different forwarded client should not be limited, got %d", code)
	}
}

// The proxy does not reach the backend from 127.0.0.1 in the deployed topology.
// Caddy runs on the host network and connects to 127.0.0.1:8080, which docker-proxy
// forwards in userland, so the container sees the bridge gateway (172.17.0.1). With
// only loopback trusted, gin discarded Caddy's X-Forwarded-For and keyed every
// proxied request to that one address: a single global bucket for all HTTPS traffic.
// Measured in production, not hypothesised.
func TestRateLimitTrustsForwardedForFromDockerBridge(t *testing.T) {
	r := limitedRouter(2, time.Minute)

	hit := func(fwd string) int {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/probe", nil)
		req.RemoteAddr = "172.17.0.1:44444" // docker-proxy, as the container sees it
		req.Header.Set("X-Forwarded-For", fwd)
		r.ServeHTTP(w, req)
		return w.Code
	}

	hit("198.51.100.20")
	hit("198.51.100.20")
	if code := hit("198.51.100.20"); code != http.StatusTooManyRequests {
		t.Fatalf("third request from the same forwarded client should be 429, got %d", code)
	}
	if code := hit("198.51.100.21"); code != http.StatusOK {
		t.Fatalf("a different client behind the same proxy must keep its own budget, got %d", code)
	}
}

// The bridge range is private and unroutable from the internet, so widening trust to
// it cannot be abused from outside. A public peer still cannot forge a client.
func TestRateLimitStillIgnoresForwardedForFromAPublicPeer(t *testing.T) {
	r := limitedRouter(2, time.Minute)

	hit := func(peer, fwd string) int {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/probe", nil)
		req.RemoteAddr = peer + ":1234"
		req.Header.Set("X-Forwarded-For", fwd)
		r.ServeHTTP(w, req)
		return w.Code
	}

	hit("198.51.100.30", "10.1.1.1")
	hit("198.51.100.30", "10.1.1.2")
	if code := hit("198.51.100.30", "10.1.1.3"); code != http.StatusTooManyRequests {
		t.Fatalf("a public peer rotating X-Forwarded-For must not gain budget, got %d", code)
	}
}
