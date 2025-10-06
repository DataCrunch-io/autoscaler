package util

import (
	mathrand "math/rand"
	"time"
)

// SeededRand is a global random instance that is seeded (for non-cryptographic use)
var SeededRand = mathrand.New(mathrand.NewSource(time.Now().UnixNano()))
