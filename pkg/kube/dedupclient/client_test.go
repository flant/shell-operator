package dedupclient

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

func TestNew_RejectsEmptyRESTConfigHost(t *testing.T) {
	t.Parallel()

	_, err := New(Config{RESTConfig: &rest.Config{}}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rest config host can't be empty")
}

// TestNew_AcceptsZeroValueOptions exercises the buildOptions translation
// path: every Config field is left at its zero value, so kubeclient must
// receive no options at all and use its built-in defaults.
func TestNew_AcceptsZeroValueOptions(t *testing.T) {
	t.Parallel()

	c, err := New(Config{RESTConfig: &rest.Config{Host: "http://127.0.0.1:0"}}, nil)
	require.NoError(t, err)
	require.NotNil(t, c)
	assert.NotNil(t, c.Dedup(), "underlying *kubeclient.DedupClient must be set")
}

func TestNew_PropagatesNonZeroOptions(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	cfg := Config{
		RESTConfig:         &rest.Config{Host: "http://127.0.0.1:0"},
		Scheme:             scheme,
		Namespaces:         []string{"kube-system"},
		WatchGVKs:          []schema.GroupVersionKind{{Group: "", Version: "v1", Kind: "Pod"}},
		ReconstructLRUSize: 128,
		GCInterval:         15 * time.Second,
	}

	c, err := New(cfg, nil)
	require.NoError(t, err)
	require.NotNil(t, c)
	// Scheme round-trip is the only directly observable Option from the
	// resulting client; the rest are stored privately by kubeclient.
	assert.Same(t, scheme, c.Scheme())
}

func TestShutdown_BeforeStart_NoOp(t *testing.T) {
	t.Parallel()

	c, err := New(Config{RESTConfig: &rest.Config{Host: "http://127.0.0.1:0"}}, nil)
	require.NoError(t, err)

	err = c.Shutdown(context.Background())
	require.NoError(t, err, "Shutdown before Start must be a no-op")
}

func TestStart_RepeatedCallsAreIdempotent(t *testing.T) {
	t.Parallel()

	c, err := New(Config{RESTConfig: &rest.Config{Host: "http://127.0.0.1:0"}}, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	require.NoError(t, c.Start(ctx))
	// A second Start must report ErrAlreadyStarted without spawning a new
	// goroutine.
	err = c.Start(ctx)
	require.ErrorIs(t, err, ErrAlreadyStarted)

	// Cancelling the parent context drains the run loop; Shutdown then
	// returns immediately because <-c.done is already closed.
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	require.NoError(t, c.Shutdown(shutdownCtx))
}

func TestWaitForCacheSync_ReturnsFalseBeforeStart(t *testing.T) {
	t.Parallel()

	c, err := New(Config{RESTConfig: &rest.Config{Host: "http://127.0.0.1:0"}}, nil)
	require.NoError(t, err)

	assert.False(t, c.WaitForCacheSync(context.Background()),
		"WaitForCacheSync must return false until Start has been called")
}
