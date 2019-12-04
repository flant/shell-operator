package kube

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_EqualOneOf(t *testing.T) {
	tests := []struct {
		name    string
		term    string
		choices []string
		result  bool
	}{
		{
			"1",
			"Pod",
			[]string{"pod", "deployment"},
			true,
		},
		{
			"2",
			"Pod",
			[]string{"pood", "deployment"},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := equalToOneOf(tt.term, tt.choices...)
			assert.Equal(t, tt.result, res)
		})
	}
}
