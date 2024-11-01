package config

import (
	"fmt"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/go_lib/log"
	"github.com/stretchr/testify/assert"
)

func TestConfig_Register(t *testing.T) {
	c := NewConfig(log.NewNop())

	c.Register("log.level", "", "info", nil, nil)

	logLevel := c.Value("log.level")
	assert.Equal(t, "info", logLevel)

	c.Set("log.level", "debug")

	logLevel = c.Value("log.level")
	assert.Equal(t, "debug", logLevel)

	c.Unset("log.level")

	logLevel = c.Value("log.level")
	assert.Equal(t, "info", logLevel)

	params := c.List()

	assert.Len(t, params, 1)
}

func TestConfig_OnChange(t *testing.T) {
	c := NewConfig(log.NewNop())

	newValue := ""
	c.Register("log.level", "", "info", func(oldValue string, n string) error {
		newValue = n
		return nil
	}, nil)

	c.Set("log.level", "debug")
	assert.Equal(t, "debug", newValue, "onChange not called for Set")

	c.Unset("log.level")
	assert.Equal(t, "info", newValue, "onChange not called for Unset after Set")

	// Temporal value
	c.SetTemporarily("log.level", "debug", 10*time.Second)
	assert.Equal(t, "debug", newValue, "onChange not called for SetTemporarily")

	c.Unset("log.level")
	assert.Equal(t, "info", newValue, "onChange not called for Unset after SetTemporarily")

	// Set after SetTemporarily
	c.SetTemporarily("log.level", "debug", 10*time.Second)
	assert.Equal(t, "debug", newValue, "onChange not called for SetTemporarily")

	c.Set("log.level", "error")
	assert.Equal(t, "error", newValue, "onChange not called for Set after SetTemporarily")

	c.Unset("log.level")
	assert.Equal(t, "info", newValue, "onChange not called for Unset after SetTemporarily+Set")
}

func TestConfig_Errors(t *testing.T) {
	var err error
	c := NewConfig(log.NewNop())

	c.Register("log.level", "", "info", func(oldValue string, n string) error {
		if n == "debug" {
			return nil
		}
		return fmt.Errorf("unknown value")
	}, nil)

	c.Set("log.level", "bad-value")
	err = c.LastError("log.level")
	assert.Error(t, err, "Set should save error about unknown value")

	c.Set("log.level", "debug")
	err = c.LastError("log.level")
	assert.NoError(t, err, "Set should clean error after success in onChange handler")
}
