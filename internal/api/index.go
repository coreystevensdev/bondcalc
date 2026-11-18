package api

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed index.html
var indexHTML []byte

// Index serves the demo page. The root used to 404, so the only URL a README
// could hand a browser was /health, which renders as a line of JSON.
func Index(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
}
