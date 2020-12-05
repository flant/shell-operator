package exponential_backoff

import (
	"math/rand"
	"testing"
	"time"
)

func Test_Delay(t *testing.T) {
	var initialSeconds int64 = 5
	initialDelay := time.Duration(5) * time.Second
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < 64; i++ {
		delay := CalculateDelay(initialDelay, i)
		seconds := int64(delay.Seconds())

		switch {
		case i == 0:
			if seconds != initialSeconds {
				t.Fatalf("delay for retry %d shoud be %d seconds, actual delay is %s", i, initialSeconds, delay)
			}
		case i <= ExponentialCalculationsCount:
			if seconds > int64(MaxExponentialBackoffDelay.Seconds()) || seconds < 0 {
				t.Fatalf("delay for retry %d shoud be greater than 0 and less than MaxExponentialBackoffDelay=%d seconds, actual delay is %s", i, int64(MaxExponentialBackoffDelay.Seconds()), delay)
			}
		case i > ExponentialCalculationsCount:
			if seconds != int64(MaxExponentialBackoffDelay.Seconds()) {
				t.Fatalf("delay for retry %d shoud be MaxExponentialBackoffDelay=%d seconds, actual delay is %s", i, int64(MaxExponentialBackoffDelay.Seconds()), delay)
			}
		}

		// debug messages
		//t.Logf("Delay for %02d retry: %s\n", i, delay)
	}
}
