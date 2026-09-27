package api

import (
	"time"

	"github.com/gin-gonic/gin"
)

// Generous enough to poke at from a README, small enough that a scripted caller
// gets nowhere. The endpoint costs CPU only, so the ceiling is about keeping one
// t3.micro responsive rather than about spend.
const (
	DemoRateLimit  = 30
	DemoRateWindow = time.Minute
)

// Only a reverse proxy on this host may be believed about who the caller is.
// Anything else reaching the port directly is keyed on its own address, which is
// what keeps the demo limiter from being bypassed with one header. Exported so the
// tests assert against the same list the server runs with, rather than a copy.
var TrustedProxies = []string{"127.0.0.1", "::1"}

func Register(r *gin.Engine) {
	if err := r.SetTrustedProxies(TrustedProxies); err != nil {
		panic("trusted proxies: " + err.Error())
	}

	r.GET("/health", Health)

	v1 := r.Group("/api/v1")

	// Unauthenticated so the README's curl runs as written. The computation
	// reads no storage and holds no secret, so there is nothing here for a
	// token to protect; the limiter, not auth, is what keeps it cheap.
	v1.POST("/demo/calculate", RateLimit(DemoRateLimit, DemoRateWindow), Calculate)

	authed := v1.Group("")
	authed.Use(JWTAuth())
	{
		authed.POST("/calculate", Calculate)
	}
}
