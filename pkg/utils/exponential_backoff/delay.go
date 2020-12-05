package exponential_backoff

import (
	"math"
	"math/rand"
	"time"
)

const MaxExponentialBackoffDelay = time.Duration(32 * time.Second)
const ExponentialDelayFactor float64 = 2.0 // Each delta delay is twice bigger.
const ExponentialDelayRandomMs = 1000      // Each delay has random additional milliseconds.

// Count of exponential calculations before return max delay to prevent overflow with big numbers.
var ExponentialCalculationsCount = int(math.Log(MaxExponentialBackoffDelay.Seconds()) / math.Log(ExponentialDelayFactor))

// CalculateDelay returns delay distributed from initialDelay to default maxDelay (32s)
//
// Example:
//   Retry 0: 5s
//   Retry 1: 6s
//   Retry 2: 7.8s
//   Retry 3: 9.8s
//   Retry 4: 13s
//   Retry 5: 21s
//   Retry 6: 32s
//   Retry 7: 32s
func CalculateDelay(initialDelay time.Duration, retryCount int) time.Duration {
	return CalculateDelayWithMax(initialDelay, MaxExponentialBackoffDelay, retryCount)
}

// CalculateDelayWithMax returns delay distributed from initialDelay to maxDelay based on retryCount number.
//
// Delay for retry number 0 is an initialDelay.
//
// Calculation of exponential delays starts from retry number 1.
//
// After ExponentialCalculationsCount rounds of calculations, maxDelay is returned.
func CalculateDelayWithMax(initialDelay time.Duration, maxDelay time.Duration, retryCount int) time.Duration {
	var delayNs int64
	switch {
	case retryCount == 0:
		return initialDelay
	case retryCount <= ExponentialCalculationsCount:
		// Calculate exponential delta for delay.
		delayNs = int64(float64(time.Second) * math.Pow(ExponentialDelayFactor, float64(retryCount-1)))
	default:
		// No calculation, return maxDelay.
		delayNs = maxDelay.Nanoseconds()
	}

	// Random addition to delay.
	rndDelayNs := rand.Int63n(ExponentialDelayRandomMs) * int64(time.Millisecond)

	delay := initialDelay + time.Duration(delayNs) + time.Duration(rndDelayNs)
	delay = delay.Truncate(100 * time.Millisecond)
	if delay.Nanoseconds() > maxDelay.Nanoseconds() {
		return maxDelay
	}
	return delay
}
