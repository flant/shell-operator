package hook

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_HookConfig_VersionedConfig_Unmarshal(t *testing.T) {

	var hookConfig *HookConfig
	var err error

	tests := []struct {
		name     string
		jsonText string
		testFn   func()
	}{
		{
			"load v0 config",
			`{"onStartup": 1}`,
			func() {
				if assert.NoError(t, err) {
					assert.Equal(t, hookConfig.Version, "v0")
					assert.NotNil(t, hookConfig.V0)
					assert.Nil(t, hookConfig.V1)
				}
			},
		},
		{
			"load v1 config",
			`{"configVersion":"v1","onStartup": 1}`,
			func() {
				if assert.NoError(t, err) {
					assert.Equal(t, hookConfig.Version, "v1")
					assert.Nil(t, hookConfig.V0)
					assert.NotNil(t, hookConfig.V1)
				}
			},
		},
		{
			"load unknown version",
			`{"configVersion":"1.0.1-unknown","onStartup": 1}`,
			func() {
				assert.Error(t, err)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			hookConfig = &HookConfig{}
			err = nil
			err = json.Unmarshal([]byte(test.jsonText), hookConfig)
			test.testFn()
		})
	}
}

// Test loading legacy version of hook configuration
func Test_HookConfig_V0(t *testing.T) {
	configData := `
{"schedule":[
{"name":"each 1 min", "crontab":"0 */1 * * * *"},
{"name":"each 5 min", "crontab":"0 */5 * * * *"}
], "onKubernetesEvent":[
{"name":"monitor pods", "kind":"pod", "allowFailure":true}
]}
`

	hc := &HookConfig{}

	err := json.Unmarshal([]byte(configData), hc)

	if assert.NoError(t, err) {
		assert.Equal(t, "v0", hc.Version)
		assert.NotNil(t, hc.V0)
		assert.Nil(t, hc.V1)
	}

	err = hc.Convert()
	if assert.NoError(t, err) {
		assert.Nil(t, hc.OnStartup)
		assert.Len(t, hc.Schedules, 2)
		assert.Len(t, hc.OnKubernetesEvents, 1)
	}

}

// Test loading v1 version of hook configuration
func Test_HookConfig_V1(t *testing.T) {
	configData := `
{"configVersion":"v1",
"schedule":[
{"name":"each 1 min", "crontab":"0 */1 * * * *"},
{"name":"each 5 min", "crontab":"0 */5 * * * *"}
], "onKubernetesEvent":[
{"name":"monitor pods", "kind":"pod", "allowFailure":true}
]}
`

	hc := &HookConfig{}

	err := json.Unmarshal([]byte(configData), hc)
	if assert.NoError(t, err) {
		assert.Equal(t, "v1", hc.Version)
		assert.Nil(t, hc.V0)
		assert.NotNil(t, hc.V1)
	}

	err = hc.Convert()
	if assert.NoError(t, err) {
		assert.Nil(t, hc.OnStartup)
		assert.Len(t, hc.Schedules, 2)
		assert.Len(t, hc.OnKubernetesEvents, 1)
	}
}
