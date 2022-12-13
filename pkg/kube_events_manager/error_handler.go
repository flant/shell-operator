package kube_events_manager

import (
	log "github.com/sirupsen/logrus"
	"io"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"

	"github.com/flant/shell-operator/pkg/metric_storage"
	utils "github.com/flant/shell-operator/pkg/utils/labels"
)

type WatchErrorHandler struct {
	description   string
	kind          string
	logEntry      *log.Entry
	metricStorage *metric_storage.MetricStorage
}

func NewWatchErrorHandler(description string, kind string, logLabels map[string]string, metricStorage *metric_storage.MetricStorage) *WatchErrorHandler {
	return &WatchErrorHandler{
		description:   description,
		kind:          kind,
		logEntry:      log.WithFields(utils.LabelsToLogFields(logLabels)),
		metricStorage: metricStorage,
	}
}

// Handler is the implementation of WatchErrorHandler that is aware of monitors and metricStorage
func (weh *WatchErrorHandler) Handler(_ *cache.Reflector, err error) {
	errorType := "nil"

	switch {
	case IsExpiredError(err):
		// Don't set LastSyncResourceVersionUnavailable - LIST call with ResourceVersion=RV already
		// has a semantic that it returns data at least as fresh as provided RV.
		// So first try to LIST with setting RV to resource version of last observed object.
		weh.logEntry.Errorf("%s: Watch of %v closed with: %v", weh.description, weh.kind, err)
		errorType = "expired"
	case err == io.EOF:
		// watch closed normally
		errorType = "eof"
	case err == io.ErrUnexpectedEOF:
		weh.logEntry.Errorf("%s: Watch for %v closed with unexpected EOF: %v", weh.description, weh.kind, err)
		errorType = "unexpected-eof"
	case err != nil:
		weh.logEntry.Errorf("%s: Failed to watch %v: %v", weh.description, weh.kind, err)
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
