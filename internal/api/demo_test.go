package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coreystevensdev/bondcalc/internal/api"
)

// A 5% 10-year semiannual bond priced at par yields 5%, which makes the demo
// route's arithmetic checkable without trusting the authed route.
var demoBody = []byte(`{"face_value":1000,"annual_coupon_rate":0.05,
  "coupons_per_year":2,"periods_remaining":20,"price":1000}`)

func postDemo(t *testing.T, ip string) *httptest.ResponseRecorder {
	t.Helper()
	r := setupRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/demo/calculate", bytes.NewReader(demoBody))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = ip + ":1234"
	r.ServeHTTP(w, req)
	return w
}

func TestDemoCalculateNeedsNoToken(t *testing.T) {
	w := postDemo(t, "203.0.113.1")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 without a token, got %d: %s", w.Code, w.Body.String())
	}

	var got map[string]float64
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v (%s)", err, w.Body.String())
	}
	if ytm := got["yield_to_maturity"]; ytm < 0.0499 || ytm > 0.0501 {
		t.Fatalf("par bond should yield its coupon rate, got %v", ytm)
	}
}

// The route is the thing that has to be limited, not just the middleware in
// isolation: wiring it to the authed group instead would leave the demo open.
func TestDemoCalculateIsRateLimited(t *testing.T) {
	r := setupRouter()

	send := func() int {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/demo/calculate", bytes.NewReader(demoBody))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "203.0.113.50:1234"
		r.ServeHTTP(w, req)
		return w.Code
	}

	for i := 0; i < api.DemoRateLimit; i++ {
		if code := send(); code != http.StatusOK {
			t.Fatalf("request %d within the limit returned %d", i+1, code)
		}
	}
	if code := send(); code != http.StatusTooManyRequests {
		t.Fatalf("request past DemoRateLimit should be 429, got %d", code)
	}
}

// The authed route must not inherit the demo limiter, or a real client's budget
// would be spent by demo traffic from the same address.
func TestAuthedCalculateIsNotDemoLimited(t *testing.T) {
	r := setupRouter()
	tok := makeToken(t)

	for i := 0; i < api.DemoRateLimit+5; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/calculate", bytes.NewReader(demoBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tok)
		req.RemoteAddr = "203.0.113.51:1234"
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("authed request %d returned %d: %s", i+1, w.Code, w.Body.String())
		}
	}
}
