package admission

import "testing"

func Test_DetectConfigurationAndWebhook(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		expect []string
	}{
		{
			"simple",
			"/azaa/qqqq",
			[]string{"azaa", "qqqq"},
		},
		{
			"composite webhookId",
			"/hooks/hook-name/my-crd-validator",
			[]string{"hooks", "hook-name/my-crd-validator"},
		},
		{
			"no webhookId",
			"/hooks/",
			[]string{"hooks", ""},
		},
		{
			"no configurationId",
			"////",
			[]string{"", ""},
		},
		{
			"empty_1",
			"",
			[]string{"", ""},
		},
		{
			"empty_2",
			"/",
			[]string{"", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, w := detectConfigurationAndWebhook(tt.path)
			if c != tt.expect[0] {
				t.Fatalf("expected configurationId '%s', got '%s'", tt.expect[0], c)
			}
			if w != tt.expect[1] {
				t.Fatalf("expected webhookId '%s', got '%s'", tt.expect[1], w)
			}
		})
	}
}
