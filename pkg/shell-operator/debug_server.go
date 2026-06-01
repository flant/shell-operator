package shell_operator

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/flant/shell-operator/pkg/config"
	"github.com/flant/shell-operator/pkg/debug"
	"github.com/flant/shell-operator/pkg/task/dump"
)

// hook path may be nested like: /hook/myfolder/myhook.sh/snapshots
var snapshotRe = regexp.MustCompile(`/hook/(.*)/snapshots.*`)

// RunDefaultDebugServer initialized and run default debug server on unix and http sockets
// This method is also used in addon-operator
func RunDefaultDebugServer(unixSocket, httpServerAddress string, logger *log.Logger) (*debug.Server, error) {
	dbgSrv := debug.NewServer("/debug", unixSocket, httpServerAddress, logger)

	dbgSrv.RegisterHandler(http.MethodGet, "/", func(_ *http.Request) (interface{}, error) {
		return "debug endpoint is alive", nil
	})

	err := dbgSrv.Init()

	return dbgSrv, err
}

// RegisterDebugQueueRoutes register routes for dumping main queue
// this method is also used in addon-operator
func (op *ShellOperator) RegisterDebugQueueRoutes(dbgSrv *debug.Server) {
	dbgSrv.RegisterHandler(http.MethodGet, "/queue/main.{format:(json|yaml|text)}", func(req *http.Request) (interface{}, error) {
		format := debug.FormatFromRequest(req)
		return dump.TaskMainQueue(op.TaskQueues, format), nil
	})

	dbgSrv.RegisterHandler(http.MethodGet, "/queue/list.{format:(json|yaml|text)}", func(req *http.Request) (interface{}, error) {
		showEmptyStr := req.URL.Query().Get("showEmpty")
		showEmpty, err := strconv.ParseBool(showEmptyStr)
		if err != nil {
			showEmpty = false
		}
		format := debug.FormatFromRequest(req)
		return dump.TaskQueues(op.TaskQueues, format, showEmpty), nil
	})
}

// RegisterDebugHookRoutes register routes for dumping queues
func (op *ShellOperator) RegisterDebugHookRoutes(dbgSrv *debug.Server) {
	dbgSrv.RegisterHandler(http.MethodGet, "/hook/list.{format:(json|yaml|text)}", func(_ *http.Request) (interface{}, error) {
		return op.HookManager.GetHookNames(), nil
	})

	// handler for dump hook snapshots
	// Example path: /hook/100-test.sh/snapshots.text
	dbgSrv.RegisterHandler(http.MethodGet, "/hook/*", func(r *http.Request) (interface{}, error) {
		// check regex match
		isMatched := snapshotRe.MatchString(r.RequestURI)
		if !isMatched {
			return nil, &debug.NotFoundError{Msg: "404 page not found"}
		}

		// Extracting hook name from URI
		matched := snapshotRe.FindStringSubmatch(r.RequestURI) // expression returns slice of: matched substring, matched group hookName
		var hookName string
		if len(matched) >= 2 { // expected presence of second element (hookName)
			hookName = matched[1]
		}
		if hookName == "" {
			return nil, &debug.BadRequestError{Msg: "'hook' parameter is required"}
		}

		// Return hook snapshot dump
		h := op.HookManager.GetHook(hookName)
		if h == nil {
			return nil, &debug.BadRequestError{Msg: fmt.Sprintf("hook '%s' is not exist", hookName)}
		}
		return h.HookController.SnapshotsDump(), nil
	})
}

// RegisterDebugDedupClientRoutes exposes a small JSON snapshot of the
// singleton deduplicated kubeclient state on the debug server.
func (op *ShellOperator) RegisterDebugDedupClientRoutes(dbgSrv *debug.Server) {
	dbgSrv.RegisterHandler(http.MethodGet, "/dedup-client/status.{format:(json|yaml|text)}", func(_ *http.Request) (interface{}, error) {
		payload := map[string]any{
			"client":        clientStatus(op),
			"snapshotStore": snapshotStoreStatus(op),
		}
		return payload, nil
	})
}

// clientStatus reports the status of the singleton dedup client.
func clientStatus(op *ShellOperator) map[string]any {
	if op.KubeClient == nil {
		return map[string]any{
			"enabled": false,
			"reason":  "KubeClient is not configured",
		}
	}
	// Cache wide synchronisation status — best-effort, capped at 0
	// timeout so the probe never blocks the debug server.
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	synced := op.KubeClient.WaitForCacheSync(ctx)
	return map[string]any{
		"enabled":         true,
		"cacheSyncedHint": synced,
	}
}

// snapshotStoreStatus reports the live counters of the shared snapshot store.
func snapshotStoreStatus(op *ShellOperator) map[string]any {
	if op.SnapshotStore == nil {
		return map[string]any{
			"enabled": false,
			"reason":  "SnapshotStore is not configured (set --dedup-client-snapshot-store or $DEDUP_CLIENT_SNAPSHOT_STORE)",
		}
	}
	stats := op.SnapshotStore.Stats()
	return map[string]any{
		"enabled":       true,
		"liveObjects":   stats.LiveObjects,
		"totalAcquires": stats.TotalAcquires,
		"totalReleases": stats.TotalReleases,
		"totalDeletes":  stats.TotalDeletes,
	}
}

// RegisterDebugConfigRoutes registers routes to manage runtime configuration.
// This method is also used in addon-operator
func (op *ShellOperator) RegisterDebugConfigRoutes(dbgSrv *debug.Server, runtimeConfig *config.Config) {
	dbgSrv.RegisterHandler(http.MethodGet, "/config/list.{format:(json|yaml|text)}", func(r *http.Request) (interface{}, error) {
		format := debug.FormatFromRequest(r)
		if format == "text" {
			return runtimeConfig.String(), nil
		}
		return runtimeConfig.List(), nil
	})

	dbgSrv.RegisterHandler(http.MethodPost, "/config/set", func(r *http.Request) (interface{}, error) {
		err := r.ParseForm()
		if err != nil {
			return nil, err
		}

		name := r.PostForm.Get("name")
		if name == "" {
			return nil, &debug.BadRequestError{Msg: "'name' parameter is required"}
		}
		if !runtimeConfig.Has(name) {
			return nil, &debug.BadRequestError{Msg: fmt.Sprintf("unknown runtime parameter %q", name)}
		}

		value := r.PostForm.Get("value")
		if name == "" {
			return nil, &debug.BadRequestError{Msg: "'value' parameter is required"}
		}

		if err = runtimeConfig.IsValid(name, value); err != nil {
			return nil, &debug.BadRequestError{Msg: fmt.Sprintf("'value' parameter is invalid: %s", err)}
		}

		var duration time.Duration
		durationStr := r.PostForm.Get("duration")
		if durationStr != "" {
			duration, err = time.ParseDuration(durationStr)
			if err != nil {
				return nil, &debug.BadRequestError{Msg: fmt.Sprintf("parse duration %q failed: %s", durationStr, err)}
			}
		}
		if duration == 0 {
			runtimeConfig.Set(name, value)
		} else {
			runtimeConfig.SetTemporarily(name, value, duration)
		}

		return nil, runtimeConfig.LastError(name)
	})
}
