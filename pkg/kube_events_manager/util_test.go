package kube_events_manager

import (
	"fmt"
	"testing"
)

func Test_RandomizedResyncPeriod(t *testing.T) {
	t.SkipNow()
	for i := 0; i < 10; i++ {
		p := randomizedResyncPeriod()
		fmt.Printf("%02d. %s\n", i, p.String())
	}
}
