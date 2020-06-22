package measure

import "time"

// Duration returns a function that can be deferred to measure an execution time of a method call
func Duration(resultCallback func(d time.Duration)) func() {
	start := time.Now()
	return func() {
		if resultCallback != nil {
			resultCallback(time.Since(start))
		}
	}
}
