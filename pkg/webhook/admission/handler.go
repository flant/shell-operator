package admission

import (
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

type EventHandlerFn func(event Event) (*Response, error)

type WebhookHandler struct {
	Router  chi.Router
	Handler EventHandlerFn
}

func NewWebhookHandler() *WebhookHandler {
	rtr := chi.NewRouter()
	h := &WebhookHandler{
		Router: rtr,
	}

	rtr.Group(func(r chi.Router) {
		r.Get("/healthz", func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(http.StatusOK)
		})
	})

	rtr.Group(func(r chi.Router) {
		r.Use(structuredLogger.NewStructuredLogger(log.NewLogger(log.Options{}).Named("admissionWebhook"), "admissionWebhook"))
		r.Use(middleware.Recoverer)
		r.Use(middleware.AllowContentType("application/json"))
		r.Post("/*", h.serveReviewRequest)
	})

	return h
}

func (h *WebhookHandler) serveReviewRequest(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var admissionReview v1.AdmissionReview
	err := json.NewDecoder(r.Body).Decode(&admissionReview)
	if err != nil {
		log.Error("failed to read admission request", log.Err(err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if admissionReview.Request == nil {
		log.Error("admission request is nil")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	admissionResponse, err := h.handleReviewRequest(r.URL.Path, admissionReview.Request)
	if err != nil {
		log.Error("validation failed", "request", admissionReview.Request.UID, log.Err(err))
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
		log.Error("Error json encoding AdmissionReview", log.Err(err))
		return
	}
}

func (h *WebhookHandler) handleReviewRequest(path string, request *v1.AdmissionRequest) (*v1.AdmissionResponse, error) {
	configurationID, webhookID := detectConfigurationAndWebhook(path)
	log.Info("Got AdmissionReview request",
		slog.String("configurationID", configurationID),
		slog.String("webhookID", webhookID))

	if h.Handler == nil {
		return nil, fmt.Errorf("AdmissionReview handler is not defined")
	}

	event := Event{
		WebhookId:       webhookID,
		ConfigurationId: configurationID,
		Request:         request,
	}

	admissionResponse, err := h.Handler(event)
	if err != nil {
		return nil, err
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
