package api

import "github.com/gin-gonic/gin"

// Register wires all routes onto the provided engine.
func Register(r *gin.Engine) {
	r.GET("/health", HandleHealth)

	v1 := r.Group("/api/v1")
	v1.Use(JWTAuth())
	{
		v1.POST("/calculate", HandleCalculate)
	}
}
