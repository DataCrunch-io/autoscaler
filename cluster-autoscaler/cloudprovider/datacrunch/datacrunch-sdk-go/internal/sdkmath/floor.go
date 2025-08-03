package sdkmath

import "math"

// Floor returns the greatest integer value less than or equal to x.
func Floor(x float64) float64 {
	return math.Floor(x)
}

// Round returns the nearest integer, rounding half away from zero.
func Round(x float64) float64 {
	return math.Round(x)
}