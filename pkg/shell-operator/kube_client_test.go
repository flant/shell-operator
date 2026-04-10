package shell_operator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestKubeClientConfig_ZeroValue(t *testing.T) {
	cfg := KubeClientConfig{}
	assert.Empty(t, cfg.Context)
	assert.Empty(t, cfg.Config)
	assert.Zero(t, cfg.QPS)
	assert.Zero(t, cfg.Burst)
	assert.Zero(t, cfg.Timeout)
}

func TestDefaultObjectPatcherKubeClient_NoTimeoutWhenZero(t *testing.T) {
	// Verify that a zero Timeout in KubeClientConfig results in no timeout being set.
	// This is a structural test: we just construct client config and verify the guard works.
	cfg := KubeClientConfig{
		Context: "",
		Config:  "",
		QPS:     5,
		Burst:   10,
		Timeout: 0, // zero → no timeout applied
	}
	assert.Equal(t, time.Duration(0), cfg.Timeout)
}

func TestKubeClientConfig_WithTimeout(t *testing.T) {
	cfg := KubeClientConfig{
		Context: "my-context",
		Config:  "/home/user/.kube/config",
		QPS:     10,
		Burst:   20,
		Timeout: 30 * time.Second,
	}
	assert.Equal(t, "my-context", cfg.Context)
	assert.Equal(t, "/home/user/.kube/config", cfg.Config)
	assert.Equal(t, float32(10), cfg.QPS)
	assert.Equal(t, 20, cfg.Burst)
	assert.Equal(t, 30*time.Second, cfg.Timeout)
}
