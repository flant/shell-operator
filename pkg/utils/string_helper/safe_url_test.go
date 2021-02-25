package string_helper

import "testing"

func Test_SafeURLString(t *testing.T) {
	tests := []struct {
		in  string
		out string
	}{
		{
			"importantHook",
			"important-hook",
		},
		{
			"hooks/nextHook",
			"hooks/next-hook",
		},
		{
			"weird spaced Name",
			"weird-spaced-name",
		},
		{
			"weird---dashed---Name",
			"weird-dashed-name",
		},
		{
			"utf8 странное имя для binding",
			"utf8-binding",
		},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			res := SafeURLString(tt.in)
			if res != tt.out {
				t.Fatalf("safeUrl should give '%s', got '%s'", tt.out, res)
			}
		})
	}
}
