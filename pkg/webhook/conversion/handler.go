package conversion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	structuredLogger "github.com/flant/shell-operator/pkg/utils/structured-logger"
	pkg "github.com/flant/shell-operator/pkg"
)

type WebhookHandler struct {
	Manager *WebhookManager
	Router  chi.Router

	Logger *log.Logger
}

func NewWebhookHandler(logger *log.Logger) *WebhookHandler {
	rtr := chi.NewRouter()
	h := &WebhookHandler{
		Router: rtr,
		Logger: logger,
	}

	rtr.Group(func(r chi.Router) {
		r.Get("/healthz", func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(http.StatusOK)
		})
	})

	rtr.Group(func(r chi.Router) {
		r.Use(structuredLogger.NewStructuredLogger(logger.Named("conversionWebhook"), "conversionWebhook"))
		r.Use(middleware.Recoverer)
		r.Use(middleware.AllowContentType("application/json"))
		r.Post("/*", h.serveReviewRequest)
	})

	return h
}

func (h *WebhookHandler) serveReviewRequest(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := r.Context()

	crdName := detectCrdName(r.URL.Path)

	logger := h.Logger.With(slog.String(pkg.LogKeyCRD, crdName))
	logger.Info("serving ConversionReview request")

	var convertReview v1.ConversionReview
	err := json.NewDecoder(r.Body).Decode(&convertReview)
	if err != nil {
		logger.Error("failed to decode ConversionReview body to json", log.Err(err))
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("Invalid JSON payload"))
		return
	}

	if convertReview.Request == nil {
		logger.Error("ConversionReview request is nil")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("Missing parameters: request"))
		return
	}

	logger = logger.With(
		slog.String(pkg.LogKeyRequest, string(convertReview.Request.UID)),
		slog.String(pkg.LogKeyKind, convertReview.Kind))

	conversionResponse, err := h.handleReviewRequest(ctx, crdName, convertReview.Request)
	if err != nil {
		logger.Error("conversion failed", log.Err(err))
		convertReview.Response = errored(err)
	} else {
		convertReview.Response = conversionResponse
	}

	convertReview.Response.UID = convertReview.Request.UID
	convertReview.Request = nil

	w.Header().Set("Content-type", "application/json")
	err = json.NewEncoder(w).Encode(convertReview)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Error json encoding ConversionReview"))
		logger.Error("error json encoding ConversionReview", log.Err(err))
		return
	}
}

// See https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/#write-a-conversion-webhook-server
// This code always response with v1 ConversionReview: it works for 1.16+.
func (h *WebhookHandler) handleReviewRequest(ctx context.Context, crdName string, request *v1.ConversionRequest) (*v1.ConversionResponse, error) {
	if h.Manager.EventHandlerFn == nil {
		return nil, fmt.Errorf("ConversionReview handler is not defined")
	}

	conversionResponse, err := h.Manager.EventHandlerFn(ctx, crdName, request)
	if err != nil {
		return nil, fmt.Errorf("handle ConversionReview: %w", err)
	}

	if conversionResponse.FailedMessage != "" {
		return nil, errors.New(conversionResponse.FailedMessage)
	}

	if len(request.Objects) != len(conversionResponse.ConvertedObjects) {
		return nil, fmt.Errorf("hook returned %d objects instead of %d", len(conversionResponse.ConvertedObjects), len(request.Objects))
	}

	return &v1.ConversionResponse{
		ConvertedObjects: conversionResponse.ConvertedObjects,
		UID:              request.UID,
		Result: metav1.Status{
			Status: metav1.StatusSuccess,
		},
	}, nil
}

// detectCrdName extracts crdName from the url path.
func detectCrdName(path string) string {
	return strings.TrimPrefix(path, "/")
}

func ExtractAPIVersions(objs []runtime.RawExtension) []string {
	verMap := make(map[string]struct{})
	res := make([]string, 0)

	for _, obj := range objs {
		var a metav1.TypeMeta
		_ = json.Unmarshal(obj.Raw, &a)

		if _, ok := verMap[a.APIVersion]; ok {
			continue
		}

		verMap[a.APIVersion] = struct{}{}
		res = append(res, a.APIVersion)
	}

	return res
}

func errored(err error) *v1.ConversionResponse {
	return &v1.ConversionResponse{
		Result: metav1.Status{
			Status:  metav1.StatusFailure,
			Message: err.Error(),
		},
	}
}
