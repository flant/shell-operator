package shell_operator

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/config"
	"github.com/flant/shell-operator/pkg/debug"
	"github.com/flant/shell-operator/pkg/task/dump"
)

func DefaultDebugServer() *debug.Server {
	dbgSrv := debug.NewServer()
	dbgSrv.WithPrefix("/debug")
	dbgSrv.WithSocketPath(app.DebugUnixSocket)
	dbgSrv.WithHttpPort(app.DebugHttpPort)
	return dbgSrv
}

func InitDefaultDebugServer() (*debug.Server, error) {
	dbgSrv := DefaultDebugServer()
	err := dbgSrv.Init()
	if err != nil {
		return nil, err
	}
	dbgSrv.Route("/", func(_ *http.Request) (interface{}, error) {
		return "debug endpoint is alive", nil
	})
	return dbgSrv, nil
}

func RegisterDebugQueueRoutes(dbgSrv *debug.Server, op *ShellOperator) {
	dbgSrv.Route("/queue/main.{format:(json|yaml|text)}", func(_ *http.Request) (interface{}, error) {
		return dump.TaskQueueMainToText(op.TaskQueues), nil
	})

	dbgSrv.Route("/queue/list.{format:(json|yaml|text)}", func(req *http.Request) (interface{}, error) {
		showEmptyStr := req.URL.Query().Get("showEmpty")
		showEmpty, err := strconv.ParseBool(showEmptyStr)
		if err != nil {
			showEmpty = false
		}
		format := debug.FormatFromRequest(req)
		return dump.TaskQueues(op.TaskQueues, format, showEmpty), nil
	})
}

func RegisterDebugHookRoutes(dbgSrv *debug.Server, op *ShellOperator) {
	dbgSrv.Route("/hook/list.{format:(json|yaml|text)}", func(_ *http.Request) (interface{}, error) {
		return op.HookManager.GetHookNames(), nil
	})

	dbgSrv.Route("/hook/{name}/snapshots.{format:(json|yaml|text)}", func(r *http.Request) (interface{}, error) {
		hookName := chi.URLParam(r, "name")
		h := op.HookManager.GetHook(hookName)
		return h.HookController.SnapshotsDump(), nil
	})
}

// RegisterDebugConfigRoutes registers routes to manage runtime configuration.
func RegisterDebugConfigRoutes(dbgSrv *debug.Server, runtimeConfig *config.Config) {
	dbgSrv.Route("/config/list.{format:(json|yaml|text)}", func(r *http.Request) (interface{}, error) {
		format := debug.FormatFromRequest(r)
		if format == "text" {
			return runtimeConfig.String(), nil
		}
		return runtimeConfig.List(), nil
	})

	dbgSrv.RoutePOST("/config/set", func(r *http.Request) (interface{}, error) {
		name := r.PostForm.Get("name")
		if name == "" {
			return nil, fmt.Errorf("'name' parameter is required")
		}
		if !runtimeConfig.Has(name) {
			return nil, fmt.Errorf("unknown runtime parameter '%s'", name)
		}

		value := r.PostForm.Get("value")
		if name == "" {
			return nil, fmt.Errorf("'value' parameter is required")
		}

		if err := runtimeConfig.IsValid(name, value); err != nil {
			return nil, fmt.Errorf("'value' parameter is invalid: %w", err)
		}

		var duration time.Duration
		var err error
		durationStr := r.PostForm.Get("duration")
		if durationStr != "" {
			duration, err = time.ParseDuration(durationStr)
			if err != nil {
				return nil, fmt.Errorf("parse duration: %v", err)
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
