package api

import (
	"net/http"

	"github.com/coreystevensdev/bondcalc/internal/bond"
	"github.com/gin-gonic/gin"
)

// CalculateRequest is the JSON body for the calculate endpoint.
type CalculateRequest struct {
	FaceValue        float64 `json:"face_value"         binding:"required,gt=0"`
	AnnualCouponRate float64 `json:"annual_coupon_rate" binding:"min=0,max=1"`
	CouponsPerYear   int     `json:"coupons_per_year"   binding:"required,min=1,max=12"`
	PeriodsRemaining int     `json:"periods_remaining"  binding:"required,min=1"`
	Price            float64 `json:"price"              binding:"required,gt=0"`
}

// HandleCalculate computes bond metrics for the supplied parameters.
func HandleCalculate(c *gin.Context) {
	var req CalculateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	b := bond.Bond{
		FaceValue:        req.FaceValue,
		AnnualCouponRate: req.AnnualCouponRate,
		CouponsPerYear:   req.CouponsPerYear,
		PeriodsRemaining: req.PeriodsRemaining,
		Price:            req.Price,
	}

	result, err := bond.Calculate(b)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// HandleHealth is a liveness probe with no external dependencies.
func HandleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
