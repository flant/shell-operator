package admission

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	v1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	structuredLogger "github.com/flant/shell-operator/pkg/utils/structured-logger"
)

type EventHandlerFn func(ctx context.Context, event Event) (*Response, error)

type WebhookHandler struct {
	Router  chi.Router
	Handler EventHandlerFn

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
		r.Use(structuredLogger.NewStructuredLogger(logger.Named("admissionWebhook"), "admissionWebhook"))
		r.Use(middleware.Recoverer)
		r.Use(middleware.AllowContentType("application/json"))
		r.Post("/*", h.serveReviewRequest)
	})

	return h
}

func (h *WebhookHandler) serveReviewRequest(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := r.Context()

	var admissionReview v1.AdmissionReview
	err := json.NewDecoder(r.Body).Decode(&admissionReview)
	if err != nil {
		h.Logger.Error("failed to decode AdmissionReview body to json", log.Err(err))
		_, _ = w.Write([]byte("Invalid JSON payload"))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if admissionReview.Request == nil {
		h.Logger.Error("AdmissionReview request is nil")
		_, _ = w.Write([]byte("Missing parameters: request"))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	logger := h.Logger.With(
		slog.String("request", string(admissionReview.Request.UID)),
		slog.String("name", admissionReview.Request.Name),
		slog.String("namespace", admissionReview.Request.Namespace),
		slog.String("kind", admissionReview.Request.Kind.Kind),
	)

	admissionResponse, err := h.handleReviewRequest(ctx, r.URL.Path, admissionReview.Request)
	if err != nil {
		logger.Error("validation failed", log.Err(err))
		admissionReview.Response = errored(err)
	} else {
		admissionReview.Response = admissionResponse
	}

	admissionReview.Response.UID = admissionReview.Request.UID
	admissionReview.Request = nil

	w.Header().Set("Content-type", "application/json")
	err = json.NewEncoder(w).Encode(admissionReview)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Error json encoding AdmissionReview"))
		logger.Error("error json encoding AdmissionReview", log.Err(err))
		return
	}
}

func (h *WebhookHandler) handleReviewRequest(ctx context.Context, path string, request *v1.AdmissionRequest) (*v1.AdmissionResponse, error) {
	configurationID, webhookID := detectConfigurationAndWebhook(path)
	h.Logger.Info("Got AdmissionReview request",
		slog.String("configurationID", configurationID),
		slog.String("webhookID", webhookID),
		slog.String("kind", request.Kind.Kind),
		slog.String("name", request.Name),
		slog.String("namespace", request.Namespace),
	)

	if h.Handler == nil {
		return nil, fmt.Errorf("AdmissionReview handler is not defined")
	}

	event := Event{
		WebhookId:       webhookID,
		ConfigurationId: configurationID,
		Request:         request,
	}

	admissionResponse, err := h.Handler(ctx, event)
	if err != nil {
		return nil, fmt.Errorf("handle AdmissionReview: %w", err)
	}

	response := &v1.AdmissionResponse{
		UID:      request.UID,
		Allowed:  admissionResponse.Allowed,
		Warnings: admissionResponse.Warnings,
		Patch:    admissionResponse.Patch,
	}

	if !admissionResponse.Allowed {
		response.Result = &metav1.Status{
			Code:    http.StatusForbidden,
			Message: admissionResponse.Message,
		}
	}

	// When allowing a request, a mutating admission webhook may optionally modify the
	// incoming object as well. This is done using the patch and patchType fields in the response.
	// The only currently supported patchType is JSONPatch. See JSON patch documentation for
	// more details. For patchType: JSONPatch, the patch field contains a base64-encoded
	// array of JSON patch operations.
	if len(admissionResponse.Patch) > 0 {
		patchType := v1.PatchTypeJSONPatch
		response.PatchType = &patchType
	}

	return response, nil
}

// detectConfigurationAndWebhook extracts configurationID and a webhookID from the url path.
func detectConfigurationAndWebhook(path string) (string /*configurationID*/, string /*webhookID*/) {
	var configurationID string
	parts := strings.Split(path, "/")
	webhookParts := make([]string, 0)
	for _, p := range parts {
		if p == "" {
			continue
		}
		if configurationID == "" {
			configurationID = p
			continue
		}
		webhookParts = append(webhookParts, p)
	}
	webhookID := strings.Join(webhookParts, "/")

	return configurationID, webhookID
}

func errored(err error) *v1.AdmissionResponse {
	return &v1.AdmissionResponse{
		Allowed: false,
		Result: &metav1.Status{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		},
	}
}
