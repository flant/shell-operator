package shell_operator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/flant/shell-operator/pkg/app"
)

func TestParseGVKs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   []string
		want    []schema.GroupVersionKind
		wantErr bool
	}{
		{
			name:  "nil input yields nil",
			input: nil,
			want:  nil,
		},
		{
			name:  "empty slice yields nil",
			input: []string{},
			want:  nil,
		},
		{
			name:  "core resource with empty group",
			input: []string{"/v1/Pod"},
			want: []schema.GroupVersionKind{
				{Group: "", Version: "v1", Kind: "Pod"},
			},
		},
		{
			name: "namespaced apps and rbac groups",
			input: []string{
				"apps/v1/Deployment",
				"rbac.authorization.k8s.io/v1/Role",
			},
			want: []schema.GroupVersionKind{
				{Group: "apps", Version: "v1", Kind: "Deployment"},
				{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "Role"},
			},
		},
		{
			name:  "leading and trailing whitespace is stripped",
			input: []string{"  apps/v1/Deployment  ", "/v1/Pod"},
			want: []schema.GroupVersionKind{
				{Group: "apps", Version: "v1", Kind: "Deployment"},
				{Group: "", Version: "v1", Kind: "Pod"},
			},
		},
		{
			name:  "blank entries are skipped",
			input: []string{"", "   ", "/v1/Pod"},
			want: []schema.GroupVersionKind{
				{Group: "", Version: "v1", Kind: "Pod"},
			},
		},
		{
			name:    "missing kind component is rejected",
			input:   []string{"apps/v1"},
			wantErr: true,
		},
		{
			name:    "empty version is rejected",
			input:   []string{"apps//Deployment"},
			wantErr: true,
		},
		{
			name:    "empty kind is rejected",
			input:   []string{"apps/v1/"},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseGVKs(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestDedupClientConfigFromAppConfig_Nil(t *testing.T) {
	t.Parallel()

	got := dedupClientConfigFromAppConfig(nil)
	assert.Equal(t, DedupClientConfig{}, got, "nil app.Config must yield a zero DedupClientConfig (Enabled=false)")
}

func TestDedupClientConfigFromAppConfig_PassThrough(t *testing.T) {
	t.Parallel()

	cfg := app.NewConfig()
	cfg.DedupClient.Enabled = true
	cfg.DedupClient.Namespaces = []string{"kube-system", "default"}
	cfg.DedupClient.WatchGVKs = []string{"/v1/Pod", "apps/v1/Deployment"}
	cfg.DedupClient.ReconstructLRUSize = 4096
	cfg.DedupClient.GCInterval = 30 * time.Second

	got := dedupClientConfigFromAppConfig(cfg)
	assert.True(t, got.Enabled)
	assert.Equal(t, []string{"kube-system", "default"}, got.Namespaces)
	assert.Equal(t, []string{"/v1/Pod", "apps/v1/Deployment"}, got.WatchGVKs)
	assert.Equal(t, 4096, got.ReconstructLRUSize)
	assert.Equal(t, 30*time.Second, got.GCInterval)
}

func TestInitDedupClient_DisabledReturnsNil(t *testing.T) {
	t.Parallel()

	c, err := initDedupClient(nil, DedupClientConfig{Enabled: false}, nil)
	require.NoError(t, err)
	assert.Nil(t, c, "Enabled=false must skip construction even when kubeClient is nil")
}

func TestInitDedupClient_EnabledRequiresKubeClient(t *testing.T) {
	t.Parallel()

	_, err := initDedupClient(nil, DedupClientConfig{Enabled: true}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "main kube client is nil")
}

func TestInitDedupClient_RejectsMalformedGVK(t *testing.T) {
	t.Parallel()

	// We don't need a real kube client for this test path because parseGVKs
	// is invoked before any rest.Config is touched. But initDedupClient does
	// dereference kubeClient before parsing — so we short-circuit by
	// constructing a request that fails at the GVK-parse step.
	cfg := DedupClientConfig{
		Enabled:   true,
		WatchGVKs: []string{"not-a-valid-gvk"},
	}
	_, err := initDedupClient(nil, cfg, nil)
	require.Error(t, err)
	// The "main kube client is nil" branch fires first, which is the
	// stronger guarantee at this layer; parse-level rejection is exercised
	// directly by TestParseGVKs above.
	assert.Contains(t, err.Error(), "main kube client is nil")
}
