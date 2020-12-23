package validating_webhook

import "testing"

func Test_safeUrlString(t *testing.T) {
	tests := []struct {
		in  string
		out string
	}{
		{
			"azazaOlolo",
			"azaza-ololo",
		},
		{
			"hooks/azazaOlolo",
			"hooks/azaza-ololo",
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
			res := safeUrlString(tt.in)
			if res != tt.out {
				t.Fatalf("safeUrl should give '%s', got '%s'", tt.out, res)
			}
		})
	}
}
