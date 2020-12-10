package validating_webhook

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	log "github.com/sirupsen/logrus"

	v1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/flant/shell-operator/pkg/utils/structured-logger"
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
	rtr.Use(structured_logger.NewStructuredLogger(log.StandardLogger()))
	rtr.Use(middleware.Recoverer)
	rtr.Use(middleware.AllowContentType("application/json"))
	rtr.Post("/*", h.ServeHTTP)

	return h
}

func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Error reading request body"))
		log.Errorf("Error reading request body: %v", err)
		return
	}

	var review v1.AdmissionReview
	err = json.Unmarshal(bodyBytes, &review)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Error parsing AdmissionReview"))
		log.Errorf("Error parsing AdmissionReview: %v", err)
		return
	}

	dumpBytes, _ := json.MarshalIndent(review, "  ", "  ")
	fmt.Printf("## Get review: \n%s\n", string(dumpBytes))

	dumpBytes, _ = json.MarshalIndent(r.Header, "  ", "  ")
	fmt.Printf("## Request headers:\n%s\n", string(dumpBytes))

	response := h.HandleReview(&review)
	admissionResponse := v1.AdmissionReview{
		TypeMeta: review.TypeMeta,
		Response: &v1.AdmissionResponse{
			UID:     review.Request.UID,
			Allowed: response.Allow,
		},
	}

	if !response.Allow && response.Result != nil {
		admissionResponse.Response.Result = &metav1.Status{
			Code:    int32(response.Result.Code),
			Message: response.Result.Message,
		}
	}

	respBytes, err := json.Marshal(admissionResponse)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Error parsing AdmissionReview"))
		log.Errorf("Error parsing AdmissionReview: %v", err)
		return
	}

	w.Header().Set("Content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(respBytes)
}

func (h *WebhookHandler) HandleReview(review *v1.AdmissionReview) *ReviewResponse {
	if h.Manager.ReviewHandlerFn != nil {
		resp, err := h.Manager.ReviewHandlerFn(review)
		if err != nil {
			return &ReviewResponse{
				Allow: false,
				Result: &ReviewResponseResult{
					Code:    403,
					Message: err.Error(),
				},
			}
		}
		return resp
	}

	return &ReviewResponse{
		Allow: false,
		Result: &ReviewResponseResult{
			Code:    403,
			Message: "AdmissionReview handler is not defined",
		},
	}
}
