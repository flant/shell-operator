package admission

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	structured_logger "github.com/flant/shell-operator/pkg/utils/structured-logger"
	. "github.com/flant/shell-operator/pkg/webhook/admission/types"
)

type AdmissionEventHandlerFn func(event AdmissionEvent) (*AdmissionResponse, error)

type WebhookHandler struct {
	Router  chi.Router
	Handler AdmissionEventHandlerFn
}

func NewWebhookHandler() *WebhookHandler {
	rtr := chi.NewRouter()
	h := &WebhookHandler{
		Router: rtr,
	}

	rtr.Get("/healthz", func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	})

	rtr.Group(func(r chi.Router) {
		rtr.Use(structured_logger.NewStructuredLogger(log.StandardLogger(), "admissionWebhook"))
		rtr.Use(middleware.Recoverer)
		rtr.Use(middleware.AllowContentType("application/json"))
		rtr.Post("/*", h.serveReviewRequest)
	})

	return h
}

func (h *WebhookHandler) serveReviewRequest(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Error reading request body"))
		log.Errorf("Error reading request body: %v", err)
		return
	}

	admissionResponse, err := h.handleReviewRequest(r.URL.Path, bodyBytes)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
		return
	}

	respBytes, err := json.Marshal(admissionResponse)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Error json encoding AdmissionReview"))
		log.Errorf("Error json encoding AdmissionReview: %v", err)
		return
	}

	w.Header().Set("Content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(respBytes)
}

func (h *WebhookHandler) handleReviewRequest(path string, body []byte) (*v1.AdmissionReview, error) {
	configurationID, webhookID := detectConfigurationAndWebhook(path)
	log.Infof("Got AdmissionReview request for confId='%s' webhookId='%s'", configurationID, webhookID)

	var review v1.AdmissionReview
	err := json.Unmarshal(body, &review)
	if err != nil {
		log.Errorf("Error parsing AdmissionReview: %v", err)
		return nil, fmt.Errorf("fail to parse AdmissionReview")
	}

	response := &v1.AdmissionReview{
		TypeMeta: review.TypeMeta,
		Response: &v1.AdmissionResponse{
			UID: review.Request.UID,
		},
	}

	if h.Handler == nil {
		response.Response.Allowed = false
		response.Response.Result = &metav1.Status{
			Code:    http.StatusInternalServerError,
			Message: "AdmissionReview handler is not defined",
		}
		return response, nil
	}

	event := AdmissionEvent{
		WebhookId:       webhookID,
		ConfigurationId: configurationID,
		Review:          &review,
	}

	admissionResponse, err := h.Handler(event)
	if err != nil {
		response.Response.Allowed = false
		response.Response.Result = &metav1.Status{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		}
		return response, nil
	}

	if len(admissionResponse.Warnings) > 0 {
		response.Response.Warnings = admissionResponse.Warnings
	}

	if !admissionResponse.Allowed {
		response.Response.Allowed = false
		response.Response.Result = &metav1.Status{
			Code:    http.StatusForbidden,
			Message: admissionResponse.Message,
		}
		return response, nil
	}

	response.Response.Allowed = true

	// When allowing a request, a mutating admission webhook may optionally modify the
	// incoming object as well. This is done using the patch and patchType fields in the response.
	// The only currently supported patchType is JSONPatch. See JSON patch documentation for
	// more details. For patchType: JSONPatch, the patch field contains a base64-encoded
	// array of JSON patch operations.
	if len(admissionResponse.Patch) > 0 {
		response.Response.Patch = admissionResponse.Patch
		patchType := v1.PatchTypeJSONPatch
		response.Response.PatchType = &patchType
	}

	return response, nil
}

// detectConfigurationAndWebhook extracts configurationID and a webhookID from the url path.
func detectConfigurationAndWebhook(path string) (configurationID string, webhookID string) {
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
	webhookID = strings.Join(webhookParts, "/")

	return
}
