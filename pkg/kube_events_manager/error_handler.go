package kubeeventsmanager

import (
	"errors"
	"io"
	"log/slog"

	"github.com/deckhouse/deckhouse/pkg/log"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"

	"github.com/flant/shell-operator/pkg/metric"
	utils "github.com/flant/shell-operator/pkg/utils/labels"
)

type WatchErrorHandler struct {
	description   string
	kind          string
	metricStorage metric.Storage

	logger *log.Logger
}

func newWatchErrorHandler(description string, kind string, logLabels map[string]string, metricStorage metric.Storage, logger *log.Logger) *WatchErrorHandler {
	return &WatchErrorHandler{
		description:   description,
		kind:          kind,
		metricStorage: metricStorage,

		logger: utils.EnrichLoggerWithLabels(logger, logLabels),
	}
}

// Handler is the implementation of WatchErrorHandler that is aware of monitors and metricStorage
func (weh *WatchErrorHandler) handler(_ *cache.Reflector, err error) {
	errorType := "nil"

	switch {
	case IsExpiredError(err):
		// Don't set LastSyncResourceVersionUnavailable - LIST call with ResourceVersion=RV already
		// has a semantic that it returns data at least as fresh as provided RV.
		// So first try to LIST with setting RV to resource version of last observed object.
		weh.logger.Error("Watch closed",
			slog.String("description", weh.description),
			slog.String("kind", weh.kind),
			log.Err(err))
		errorType = "expired"
	case err == io.EOF:
		// watch closed normally
		errorType = "eof"
	case errors.Is(err, io.ErrUnexpectedEOF):
		weh.logger.Error("Watch closed with unexpected EOF",
			slog.String("description", weh.description),
			slog.String("kind", weh.kind),
			log.Err(err))
		errorType = "unexpected-eof"
	case err != nil:
		weh.logger.Error("Watch Failed",
			slog.String("description", weh.description),
			slog.String("kind", weh.kind),
			log.Err(err))
		errorType = "fail"
	}

	if weh.metricStorage != nil {
		weh.metricStorage.CounterAdd("{PREFIX}kubernetes_client_watch_errors_total", 1.0, map[string]string{"error_type": errorType})
	}
}

// IsExpiredError is a private method from k8s.io/client-go/tools/cache.
func IsExpiredError(err error) bool {
	// In Kubernetes 1.17 and earlier, the api server returns both apierrors.StatusReasonExpired and
	// apierrors.StatusReasonGone for HTTP 410 (Gone) status code responses. In 1.18 the kube server is more consistent
	// and always returns apierrors.StatusReasonExpired. For backward compatibility we can only remove the apierrors.IsGone
	// check when we fully drop support for Kubernetes 1.17 servers from reflectors.
	return apierrors.IsResourceExpired(err) || apierrors.IsGone(err)
}
