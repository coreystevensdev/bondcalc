package api

import "github.com/gin-gonic/gin"

func Register(r *gin.Engine) {
	r.GET("/health", Health)

	v1 := r.Group("/api/v1")
	v1.Use(JWTAuth())
	{
		v1.POST("/calculate", Calculate)
	}
}
