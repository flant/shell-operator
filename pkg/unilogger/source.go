package unilogger

import (
	"fmt"
	"sync/atomic"
)

type AddSourceVar struct {
	val atomic.Pointer[bool]
}

func (v *AddSourceVar) Source() *bool {
	return v.val.Load()
}

func (v *AddSourceVar) Set(b bool) {
	v.val.Store(&b)
}

func (v *AddSourceVar) String() string {
	return fmt.Sprintf("AddSourceVar(%t)", *v.Source())
}
