package measure

import "time"

// MeasureTimeMs is a helper to measure execution time in whole number of milliseconds
func MeasureTimeMs(resultCallback func(ms float64)) func() {
	return MeasureTime(func(nanos Nanos) {
		if resultCallback != nil {
			resultCallback(nanos.Ms())
		}
	})
}

// MeasureTime returns a function that can be defered to measure an execution time of a method call
func MeasureTime(resultCallback func(nanos Nanos)) func() {
	start := time.Now()
	return func() {
		if resultCallback != nil {
			resultCallback(Nanos(time.Since(start)))
		}
	}
}

type Nanos int64

// Micro is a whole number of microseconds
func (n Nanos) Micro() float64 {
	return float64(n / 1e3)
}

// Ms is a whole number of milliseconds
func (n Nanos) Ms() float64 {
	return float64(n / 1e6)
}

// Ms2 is a milliseconds with 2 fractional digits
func (n Nanos) Ms2() float64 {
	return float64(n/1e4) / 100
}

// Sec is a whole number of seconds
func (n Nanos) Sec() float64 {
	return float64(n / 1e9)
}

// Sec2 is a seconds with 2 fractional digits
func (n Nanos) Sec2() float64 {
	return float64(n/1e7) / 100
}
