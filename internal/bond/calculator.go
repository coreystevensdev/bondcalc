// Package bond implements fixed-income calculations: yield to maturity,
// current yield, Macaulay duration, and modified duration.
package bond

import (
	"errors"
	"math"
)

// ErrInvalidInput is returned when caller-supplied values violate domain constraints.
var ErrInvalidInput = errors.New("invalid bond parameters")

// ErrDidNotConverge is returned when the Newton-Raphson YTM solver exhausts
// its iteration budget without finding a stable root.
var ErrDidNotConverge = errors.New("yield to maturity solver did not converge")

// below this, a Newton step divides by a near-zero derivative and blows up toward Inf
const derivativeEpsilon = 1e-12

type Bond struct {
	// FaceValue is the par value paid at maturity (e.g. 1000.00).
	FaceValue float64
	// AnnualCouponRate is the stated coupon rate as a decimal (e.g. 0.05 = 5%).
	AnnualCouponRate float64
	// CouponsPerYear is the payment frequency (1=annual, 2=semi-annual, 4=quarterly).
	CouponsPerYear int
	// PeriodsRemaining is the total number of coupon periods left until maturity.
	PeriodsRemaining int
	// Price is the current market price.
	Price float64
}

type Result struct {
	CurrentYield   float64 `json:"current_yield"`
	YieldToMaturity float64 `json:"yield_to_maturity"`
	MacaulayDuration float64 `json:"macaulay_duration_years"`
	ModifiedDuration float64 `json:"modified_duration_years"`
	CouponPayment  float64 `json:"coupon_payment"`
}

func (b Bond) validate() error {
	switch {
	case b.FaceValue <= 0:
		return ErrInvalidInput
	case b.AnnualCouponRate < 0 || b.AnnualCouponRate > 1:
		return ErrInvalidInput
	case b.CouponsPerYear <= 0:
		return ErrInvalidInput
	case b.PeriodsRemaining <= 0:
		return ErrInvalidInput
	case b.Price <= 0:
		return ErrInvalidInput
	}
	return nil
}

func (b Bond) couponPerPeriod() float64 {
	return b.FaceValue * b.AnnualCouponRate / float64(b.CouponsPerYear)
}

func (b Bond) CurrentYield() (float64, error) {
	if err := b.validate(); err != nil {
		return 0, err
	}
	return (b.FaceValue * b.AnnualCouponRate) / b.Price, nil
}

// YieldToMaturity solves for the periodic yield r such that the present value
// of all future cash flows equals the current price. Newton-Raphson converges
// in ~10 iterations for typical bond parameters.
func (b Bond) YieldToMaturity() (float64, error) {
	if err := b.validate(); err != nil {
		return 0, err
	}

	coupon := b.couponPerPeriod()
	n := float64(b.PeriodsRemaining)

	// Initial guess: current yield / periods per year.
	r := (b.AnnualCouponRate / float64(b.CouponsPerYear))
	if r == 0 {
		r = 0.01
	}

	converged := false
	for i := 0; i < 200; i++ {
		discount := math.Pow(1+r, n)
		pv := coupon*(1-1/discount)/r + b.FaceValue/discount
		// dPV/dr
		dpv := -coupon*(1-1/discount)/(r*r) +
			coupon*n/((r*discount*(1+r))) +
			-b.FaceValue*n/(discount*(1+r))

		if math.Abs(dpv) < derivativeEpsilon {
			return 0, ErrDidNotConverge
		}

		delta := (pv - b.Price) / dpv
		r -= delta
		if math.IsNaN(r) || math.IsInf(r, 0) {
			return 0, ErrDidNotConverge
		}
		if math.Abs(delta) < 1e-10 {
			converged = true
			break
		}
	}

	if !converged {
		return 0, ErrDidNotConverge
	}

	return r * float64(b.CouponsPerYear), nil
}

// MacaulayDuration returns the weighted average time (in years) to receive
// the bond's cash flows, discounted at the yield to maturity.
func (b Bond) MacaulayDuration() (float64, error) {
	ytm, err := b.YieldToMaturity()
	if err != nil {
		return 0, err
	}

	coupon := b.couponPerPeriod()
	periodicYield := ytm / float64(b.CouponsPerYear)
	n := b.PeriodsRemaining

	var numerator, denominator float64
	for t := 1; t <= n; t++ {
		cf := coupon
		if t == n {
			cf += b.FaceValue
		}
		pv := cf / math.Pow(1+periodicYield, float64(t))
		timeInYears := float64(t) / float64(b.CouponsPerYear)
		numerator += timeInYears * pv
		denominator += pv
	}

	if denominator == 0 {
		return 0, ErrInvalidInput
	}
	return numerator / denominator, nil
}

// ModifiedDuration is Macaulay duration divided by (1 + periodic yield).
// It approximates the percentage price change for a 1% change in yield.
func (b Bond) ModifiedDuration() (float64, error) {
	mac, err := b.MacaulayDuration()
	if err != nil {
		return 0, err
	}
	ytm, err := b.YieldToMaturity()
	if err != nil {
		return 0, err
	}

	periodicYield := ytm / float64(b.CouponsPerYear)
	return mac / (1 + periodicYield), nil
}

func Calculate(b Bond) (Result, error) {
	cy, err := b.CurrentYield()
	if err != nil {
		return Result{}, err
	}
	ytm, err := b.YieldToMaturity()
	if err != nil {
		return Result{}, err
	}
	mac, err := b.MacaulayDuration()
	if err != nil {
		return Result{}, err
	}
	mod, err := b.ModifiedDuration()
	if err != nil {
		return Result{}, err
	}
	return Result{
		CurrentYield:     round(cy, 6),
		YieldToMaturity:  round(ytm, 6),
		MacaulayDuration: round(mac, 4),
		ModifiedDuration: round(mod, 4),
		CouponPayment:    round(b.couponPerPeriod(), 4),
	}, nil
}

func round(v float64, places int) float64 {
	factor := math.Pow(10, float64(places))
	return math.Round(v*factor) / factor
}
