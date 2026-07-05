package bond_test

import (
	"math"
	"testing"

	"github.com/coreystevensdev/bondcalc/internal/bond"
)

// Standard 10-year, 5% annual coupon bond at par.
var parBond = bond.Bond{
	FaceValue:        1000,
	AnnualCouponRate: 0.05,
	CouponsPerYear:   2,
	PeriodsRemaining: 20,
	Price:            1000,
}

// Premium bond: 6% coupon, priced above par.
var premiumBond = bond.Bond{
	FaceValue:        1000,
	AnnualCouponRate: 0.06,
	CouponsPerYear:   2,
	PeriodsRemaining: 20,
	Price:            1100,
}

// Discount bond: 4% coupon, priced below par.
var discountBond = bond.Bond{
	FaceValue:        1000,
	AnnualCouponRate: 0.04,
	CouponsPerYear:   2,
	PeriodsRemaining: 20,
	Price:            900,
}

func approxEqual(a, b, tolerance float64) bool {
	return math.Abs(a-b) < tolerance
}

func TestCurrentYield_AtPar(t *testing.T) {
	cy, err := parBond.CurrentYield()
	if err != nil {
		t.Fatal(err)
	}
	if !approxEqual(cy, 0.05, 1e-6) {
		t.Errorf("current yield at par: want 0.05, got %f", cy)
	}
}

func TestCurrentYield_PremiumBond(t *testing.T) {
	cy, err := premiumBond.CurrentYield()
	if err != nil {
		t.Fatal(err)
	}
	// 6% coupon / 110 price = 5.4545...%
	if !approxEqual(cy, 0.054545, 1e-4) {
		t.Errorf("current yield (premium): want ~0.0545, got %f", cy)
	}
}

func TestYieldToMaturity_AtPar(t *testing.T) {
	ytm, err := parBond.YieldToMaturity()
	if err != nil {
		t.Fatal(err)
	}
	// Bond priced at par: YTM == coupon rate.
	if !approxEqual(ytm, 0.05, 1e-6) {
		t.Errorf("YTM at par: want 0.05, got %f", ytm)
	}
}

func TestYieldToMaturity_PremiumBond(t *testing.T) {
	ytm, err := premiumBond.YieldToMaturity()
	if err != nil {
		t.Fatal(err)
	}
	// Premium bond: YTM < coupon rate.
	if ytm >= 0.06 {
		t.Errorf("YTM of premium bond must be < coupon rate, got %f", ytm)
	}
}

func TestYieldToMaturity_DiscountBond(t *testing.T) {
	ytm, err := discountBond.YieldToMaturity()
	if err != nil {
		t.Fatal(err)
	}
	// Discount bond: YTM > coupon rate.
	if ytm <= 0.04 {
		t.Errorf("YTM of discount bond must be > coupon rate, got %f", ytm)
	}
}

func TestMacaulayDuration_AtPar(t *testing.T) {
	mac, err := parBond.MacaulayDuration()
	if err != nil {
		t.Fatal(err)
	}
	// 10-year semi-annual 5% at-par bond: Macaulay duration ~7.99 years.
	if !approxEqual(mac, 7.99, 0.05) {
		t.Errorf("Macaulay duration at par: want ~7.99, got %f", mac)
	}
}

func TestModifiedDuration_LessThanMacaulay(t *testing.T) {
	mac, err := parBond.MacaulayDuration()
	if err != nil {
		t.Fatal(err)
	}
	mod, err := parBond.ModifiedDuration()
	if err != nil {
		t.Fatal(err)
	}
	if mod >= mac {
		t.Errorf("modified duration (%f) must be < Macaulay duration (%f)", mod, mac)
	}
}

func TestCalculate_ReturnsAllMetrics(t *testing.T) {
	r, err := bond.Calculate(parBond)
	if err != nil {
		t.Fatal(err)
	}
	if r.CurrentYield == 0 {
		t.Error("CurrentYield should be non-zero")
	}
	if r.YieldToMaturity == 0 {
		t.Error("YieldToMaturity should be non-zero")
	}
	if r.MacaulayDuration == 0 {
		t.Error("MacaulayDuration should be non-zero")
	}
	if r.ModifiedDuration == 0 {
		t.Error("ModifiedDuration should be non-zero")
	}
	if r.CouponPayment != 25.0 {
		t.Errorf("CouponPayment: want 25.00, got %f", r.CouponPayment)
	}
}

func TestValidation_NegativeFaceValue(t *testing.T) {
	b := bond.Bond{FaceValue: -100, AnnualCouponRate: 0.05, CouponsPerYear: 2, PeriodsRemaining: 10, Price: 100}
	if _, err := b.YieldToMaturity(); err == nil {
		t.Error("expected error for negative face value")
	}
}

func TestValidation_ZeroPrice(t *testing.T) {
	b := bond.Bond{FaceValue: 1000, AnnualCouponRate: 0.05, CouponsPerYear: 2, PeriodsRemaining: 10, Price: 0}
	if _, err := b.YieldToMaturity(); err == nil {
		t.Error("expected error for zero price")
	}
}

func TestValidation_ZeroCouponsPerYear(t *testing.T) {
	b := bond.Bond{FaceValue: 1000, AnnualCouponRate: 0.05, CouponsPerYear: 0, PeriodsRemaining: 10, Price: 1000}
	if _, err := b.YieldToMaturity(); err == nil {
		t.Error("expected error for zero coupons per year")
	}
}

func TestValidation_NegativeCouponRate(t *testing.T) {
	b := bond.Bond{FaceValue: 1000, AnnualCouponRate: -0.05, CouponsPerYear: 2, PeriodsRemaining: 10, Price: 1000}
	if _, err := b.YieldToMaturity(); err == nil {
		t.Error("expected error for negative coupon rate")
	}
}

func TestZeroCoupon_YTM(t *testing.T) {
	// Zero-coupon bond: no periodic payments, all value at maturity.
	zc := bond.Bond{
		FaceValue:        1000,
		AnnualCouponRate: 0.0,
		CouponsPerYear:   1,
		PeriodsRemaining: 10,
		Price:            613.91, // PV of $1000 in 10 years at 5%
	}
	ytm, err := zc.YieldToMaturity()
	if err != nil {
		t.Fatal(err)
	}
	if !approxEqual(ytm, 0.05, 0.001) {
		t.Errorf("zero-coupon YTM: want ~0.05, got %f", ytm)
	}
}
