package sdkrand

import (
	"math/rand"
	"time"
)

// SeededRand is a global random instance that is seeded
var SeededRand = rand.New(rand.NewSource(time.Now().UnixNano()))