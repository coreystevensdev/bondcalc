package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/coreystevensdev/bondcalc/internal/api"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret-key-at-least-32-chars!!"

func setupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	os.Setenv("JWT_SECRET", testSecret)
	r := gin.New()
	api.Register(r)
	return r
}

func makeToken(t *testing.T) string {
	t.Helper()
	claims := jwt.MapClaims{"sub": "test-user", "exp": time.Now().Add(time.Hour).Unix()}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return tok
}

func TestHealthReturns200(t *testing.T) {
	r := setupRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestCalculateNoAuthHeader401(t *testing.T) {
	r := setupRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/calculate", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestCalculateInvalidToken401(t *testing.T) {
	r := setupRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/calculate", nil)
	req.Header.Set("Authorization", "Bearer not-a-valid-jwt")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestCalculateMissingBody400(t *testing.T) {
	r := setupRouter()
	tok := makeToken(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/calculate", bytes.NewBufferString(`{}`))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCalculateValidRequest200(t *testing.T) {
	r := setupRouter()
	tok := makeToken(t)

	body := map[string]any{
		"face_value":         1000.0,
		"annual_coupon_rate": 0.05,
		"coupons_per_year":   2,
		"periods_remaining":  10,
		"price":              980.0,
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/calculate", bytes.NewBuffer(b))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if w.Body.Len() == 0 {
		t.Fatal("expected non-empty response body")
	}
}
