package validating

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	log "github.com/sirupsen/logrus"

	v1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/flant/shell-operator/pkg/utils/structured-logger"
	. "github.com/flant/shell-operator/pkg/webhook/validating/types"
)

type WebhookHandler struct {
	Manager *WebhookManager
	Router  chi.Router
}

func NewWebhookHandler() *WebhookHandler {
	rtr := chi.NewRouter()
	h := &WebhookHandler{
		Router: rtr,
	}
	rtr.Use(structured_logger.NewStructuredLogger(log.StandardLogger(), "validatingWebhook"))
	rtr.Use(middleware.Recoverer)
	rtr.Use(middleware.AllowContentType("application/json"))
	rtr.Post("/*", h.ServeReviewRequest)

	return h
}

func (h *WebhookHandler) ServeReviewRequest(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Error reading request body"))
		log.Errorf("Error reading request body: %v", err)
		return
	}

	admissionResponse, err := h.HandleReviewRequest(r.URL.Path, bodyBytes)
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

func (h *WebhookHandler) HandleReviewRequest(path string, body []byte) (*v1.AdmissionReview, error) {
	configurationID, webhookID := DetectConfigurationAndWebhook(path)
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

	if h.Manager.ValidatingEventHandlerFn == nil {
		response.Response.Allowed = false
		response.Response.Result = &metav1.Status{
			Code:    500,
			Message: "AdmissionReview handler is not defined",
		}
		return response, nil
	}

	event := ValidatingEvent{
		WebhookId:       webhookID,
		ConfigurationId: configurationID,
		Review:          &review,
	}

	validatingResponse, err := h.Manager.ValidatingEventHandlerFn(event)
	if err != nil {
		response.Response.Allowed = false
		response.Response.Result = &metav1.Status{
			Code:    500,
			Message: err.Error(),
		}
		return response, nil
	}

	if !validatingResponse.Allowed {
		response.Response.Allowed = false
		response.Response.Result = &metav1.Status{
			Code:    403,
			Message: validatingResponse.Message,
		}
		return response, nil
	}

	response.Response.Allowed = true
	return response, nil
}

// DetectConfigurationAndWebhook extracts configurationID and a webhookID from the url path.
func DetectConfigurationAndWebhook(path string) (configurationID string, webhookID string) {
	parts := strings.Split(path, "/")
	webhookParts := []string{}
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
