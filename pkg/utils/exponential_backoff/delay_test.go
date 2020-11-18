package exponential_backoff

import (
	"math/rand"
	"testing"
	"time"
)

func Test_Delay(t *testing.T) {
	t.SkipNow()
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < 16; i++ {
		t.Logf("Delay for %02d retry: %s\n",
			i,
			CalculateDelay(5*time.Second, i),
		)
	}
}
