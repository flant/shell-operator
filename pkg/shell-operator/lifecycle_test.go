// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package shell_operator

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// pickFreePort returns an OS-assigned free TCP port as a string. The listener
// is closed before the port is returned, so a brief race with another process
// is possible — acceptable for in-process restart tests.
func pickFreePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()
	_, portStr, err := net.SplitHostPort(l.Addr().String())
	require.NoError(t, err)
	if _, err := strconv.Atoi(portStr); err != nil {
		t.Fatalf("unexpected port %q: %v", portStr, err)
	}
	return portStr
}

// buildMinimalOperator builds a ShellOperator with just the APIServer wired up.
// All other components (HookManager, ManagerEventsHandler, KubeEventsManager,
// ScheduleManager, webhook managers, debug server) stay nil so we can exercise
// the lifecycle without a real Kubernetes cluster.
func buildMinimalOperator(t *testing.T, addr, port string) *ShellOperator {
	t.Helper()
	op := NewBareShellOperator(context.Background(), WithLogger(log.NewNop()))
	op.APIServer = newBaseHTTPServer(addr, port, log.NewNop())
	op.APIServer.RegisterRoute(http.MethodGet, "/healthz",
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	return op
}

// waitForServer polls /healthz until the server answers or the deadline elapses.
func waitForServer(t *testing.T, port string, deadline time.Duration) {
	t.Helper()
	client := &http.Client{Timeout: 200 * time.Millisecond}
	url := fmt.Sprintf("http://127.0.0.1:%s/healthz", port)
	end := time.Now().Add(deadline)
	for time.Now().Before(end) {
		resp, err := client.Get(url)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("server on port %s never became ready", port)
}

// TestShutdownStopsAllGoroutines verifies that Start spawns a known set of
// goroutines and Shutdown reliably joins them all. We snapshot goroutines
// before Start and ask goleak to verify the same baseline after Shutdown.
func TestShutdownStopsAllGoroutines(t *testing.T) {
	defer goleak.VerifyNone(t,
		// metric storage registers a prometheus collector that may keep an
		// internal goroutine alive between tests; ignore the well-known
		// background routines that are not owned by ShellOperator.
		goleak.IgnoreTopFunction("k8s.io/klog/v2.(*loggingT).flushDaemon"),
	)

	op := buildMinimalOperator(t, "127.0.0.1", pickFreePort(t))

	require.NoError(t, op.Start(context.Background()))
	waitForServer(t, op.APIServer.port, 2*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, op.Shutdown(ctx))
}

// TestRestartInSameProcess starts op1, shuts it down, and then starts op2 on
// the same port. The second Start must succeed — proof that the first
// instance released its listener cleanly.
func TestRestartInSameProcess(t *testing.T) {
	port := pickFreePort(t)

	op1 := buildMinimalOperator(t, "127.0.0.1", port)
	require.NoError(t, op1.Start(context.Background()))
	waitForServer(t, port, 2*time.Second)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, op1.Shutdown(shutdownCtx))

	op2 := buildMinimalOperator(t, "127.0.0.1", port)
	require.NoError(t, op2.Start(context.Background()))
	waitForServer(t, port, 2*time.Second)

	shutdownCtx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	require.NoError(t, op2.Shutdown(shutdownCtx2))
}

// TestShutdownIdempotent verifies that calling Shutdown twice is safe and
// the second call is a no-op that returns the original error (nil here).
func TestShutdownIdempotent(t *testing.T) {
	op := buildMinimalOperator(t, "127.0.0.1", pickFreePort(t))
	require.NoError(t, op.Start(context.Background()))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, op.Shutdown(ctx))

	// Second call: shutdownOnce should suppress all the underlying work and
	// just return the cached error.
	require.NoError(t, op.Shutdown(ctx))

	// And once shutdownOnce has fired, a third call from another goroutine
	// must also be safe.
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = op.Shutdown(ctx)
		}()
	}
	wg.Wait()
}

// blockingShutdowner pretends to be a debug server whose Shutdown never
// returns. It lets us prove that ShellOperator.Shutdown honors ctx deadlines
// instead of blocking forever on a misbehaving dependency.
type blockingShutdowner struct{}

func (blockingShutdowner) Shutdown(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

// TestShutdownHonorsContextDeadline plugs in a DebugServer that blocks on
// ctx.Done and asserts that Shutdown still returns within the deadline,
// reporting the wrapped timeout error.
func TestShutdownHonorsContextDeadline(t *testing.T) {
	op := buildMinimalOperator(t, "127.0.0.1", pickFreePort(t))
	op.DebugServer = blockingShutdowner{}

	require.NoError(t, op.Start(context.Background()))

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := op.Shutdown(ctx)
	elapsed := time.Since(start)

	require.Error(t, err, "expected a timeout error")
	assert.Contains(t, strings.ToLower(err.Error()), "debug server")
	// We don't pin the bound tight because CI machines vary, but the call
	// must not block for an order of magnitude beyond the deadline.
	assert.Less(t, elapsed, 2*time.Second, "shutdown took too long: %s", elapsed)
}

// TestStartIsIdempotent ensures Start can be called multiple times without
// double-binding the listener or spawning extra goroutines. The first call
// wins; subsequent calls return the cached error (nil here).
func TestStartIsIdempotent(t *testing.T) {
	op := buildMinimalOperator(t, "127.0.0.1", pickFreePort(t))
	require.NoError(t, op.Start(context.Background()))
	require.NoError(t, op.Start(context.Background()))
	require.NoError(t, op.Start(context.Background()))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, op.Shutdown(ctx))
}
