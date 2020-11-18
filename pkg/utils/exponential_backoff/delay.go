package exponential_backoff

import (
	"math"
	"math/rand"
	"time"
)

const MaxExponentialBackoffDelay = time.Duration(32 * time.Second)
const ExponentialDelayFactor float64 = 2.0 // Each delay is twice longer.
const ExponentialDelayRandomMs = 1000      // Each delay has random additional milliseconds.

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

func CalculateDelayWithMax(initialDelay time.Duration, maxDelay time.Duration, retryCount int) time.Duration {
	if retryCount == 0 {
		return initialDelay
	}
	delayNs := int64(float64(time.Second) * math.Pow(ExponentialDelayFactor, float64(retryCount-1)))
	rndDelayMs := rand.Intn(ExponentialDelayRandomMs)
	delay := initialDelay + time.Duration(delayNs) + time.Duration(rndDelayMs)*time.Millisecond
	delay = delay.Truncate(100 * time.Millisecond)
	if delay.Nanoseconds() > maxDelay.Nanoseconds() {
		return maxDelay
	}
	return delay
}
