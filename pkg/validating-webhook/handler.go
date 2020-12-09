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
	"sigs.k8s.io/yaml"

	structured_logger "github.com/flant/shell-operator/pkg/utils/structured-logger"
)

type WebhookHandler struct {
	Router chi.Router
}

func NewWebhookHandler() *WebhookHandler {
	rtr := chi.NewRouter()
	h := &WebhookHandler{
		Router: rtr,
	}
	rtr.Use(structured_logger.NewStructuredLogger(log.StandardLogger()))
	rtr.Use(middleware.Recoverer)
	rtr.Post("/", h.handleRequest)

	return h
}

func (h *WebhookHandler) handleRequest(w http.ResponseWriter, r *http.Request) {
	cntType := r.Header.Get("Content-type")
	log.Infof("Got request: URL:'%s' Content-type:%s", r.URL.String(), cntType)

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

	yamlBytes, _ := yaml.Marshal(review)
	fmt.Printf("## Get review: \n%s\n", string(yamlBytes))

	yamlBytes, _ = yaml.Marshal(r.Header)
	fmt.Printf("## Request headers:\n%s\n", string(yamlBytes))

	response := v1.AdmissionReview{
		TypeMeta: review.TypeMeta,
		Response: &v1.AdmissionResponse{
			UID:     review.Request.UID,
			Allowed: false,
			Result: &metav1.Status{
				Message: "You cannot do this because it is Tuesday and your name starts with A",
				Code:    403,
			},
		},
	}

	respBytes, err := json.Marshal(response)

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
