package util

import "math"

// Floor returns the greatest integer value less than or equal to x.
func Floor(x float64) float64 {
	return math.Floor(x)
}

// Round returns the nearest integer, rounding half away from zero.
func Round(x float64) float64 {
	return math.Round(x)
}

// Ceil returns the least integer value greater than or equal to x.
func Ceil(x float64) float64 {
	return math.Ceil(x)
}

// Abs returns the absolute value of x.
func Abs(x float64) float64 {
	return math.Abs(x)
}

// Max returns the larger of x or y.
func Max(x, y float64) float64 {
	return math.Max(x, y)
}

// Min returns the smaller of x or y.
func Min(x, y float64) float64 {
	return math.Min(x, y)
}
