package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coreystevensdev/bondcalc/internal/api"
)

// The deployed root used to 404, so the only link a README could offer a browser
// was /health, which renders as a line of JSON.
func TestIndexServesHTML(t *testing.T) {
	r := setupRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET / = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	body := w.Body.String()
	for _, want := range []string{"<!doctype html", "bondcalc", "/api/v1/demo/calculate"} {
		if !strings.Contains(strings.ToLower(body), strings.ToLower(want)) {
			t.Errorf("body missing %q", want)
		}
	}
}

// The page posts to the demo route, so it has to name the same fields the
// handler binds. A rename on either side breaks the form silently otherwise.
func TestIndexNamesEveryRequestField(t *testing.T) {
	r := setupRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	for _, field := range []string{
		"face_value", "annual_coupon_rate", "coupons_per_year",
		"periods_remaining", "price",
	} {
		if !strings.Contains(body, field) {
			t.Errorf("page never mentions request field %q", field)
		}
	}
	for _, field := range []string{
		"current_yield", "yield_to_maturity",
		"macaulay_duration_years", "modified_duration_years", "coupon_payment",
	} {
		if !strings.Contains(body, field) {
			t.Errorf("page never reads response field %q", field)
		}
	}
}

// Serving the page must not put it behind the demo limiter, or a browser
// refresh spends the same 30/min budget the API calls need.
func TestIndexIsNotRateLimited(t *testing.T) {
	r := setupRouter()
	for i := 0; i < api.DemoRateLimit+5; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "203.0.113.9:1234"
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("GET / #%d = %d, want 200", i+1, w.Code)
		}
	}
}
