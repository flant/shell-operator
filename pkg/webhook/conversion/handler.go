package conversion

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	structured_logger "github.com/flant/shell-operator/pkg/utils/structured-logger"
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

	rtr.Group(func(r chi.Router) {
		r.Get("/healthz", func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(http.StatusOK)
		})
	})

	rtr.Group(func(r chi.Router) {
		r.Use(structured_logger.NewStructuredLogger(log.StandardLogger(), "conversionWebhook"))
		r.Use(middleware.Recoverer)
		r.Use(middleware.AllowContentType("application/json"))
		r.Post("/*", h.serveReviewRequest)
	})

	return h
}

func (h *WebhookHandler) serveReviewRequest(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	conversionResponse, err := h.handleReviewRequest(r.URL.Path, r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	conversionResponse.Request = nil

	w.Header().Set("Content-type", "application/json")
	err = json.NewEncoder(w).Encode(conversionResponse)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Error json encoding ConversionReview"))
		log.Errorf("Error json encoding ConversionReview: %v", err)
		return
	}
}

// See https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/#write-a-conversion-webhook-server
// This code always response with v1 ConversionReview: it works for 1.16+.
func (h *WebhookHandler) handleReviewRequest(path string, body io.Reader) (*v1.ConversionReview, error) {
	crdName := detectCrdName(path)
	log.Infof("Got ConversionReview request for crd/%s", crdName)

	var inReview v1.ConversionReview
	err := json.NewDecoder(body).Decode(&inReview)
	if err != nil {
		log.Errorf("Error parsing ConversionReview: %v", err)
		return nil, fmt.Errorf("fail to parse ConversionReview: %w", err)
	}

	if inReview.Request == nil {
		return nil, fmt.Errorf("conversion request is nil")
	}

	inReview.Response.UID = inReview.Request.UID

	if h.Manager.EventHandlerFn == nil {
		inReview.Response.Result = metav1.Status{
			Status:  "Failed",
			Message: "ConversionReview handler is not defined",
		}
		return &inReview, nil
	}

	conversionResponse, err := h.Manager.EventHandlerFn(crdName, &inReview)
	if err != nil {
		inReview.Response.Result = metav1.Status{
			Status:  "Failed",
			Message: err.Error(),
		}
		return &inReview, nil
	}

	if conversionResponse.FailedMessage != "" {
		inReview.Response.Result = metav1.Status{
			Status:  "Failed",
			Message: conversionResponse.FailedMessage,
		}
		return &inReview, nil
	}

	if len(inReview.Request.Objects) != len(conversionResponse.ConvertedObjects) {
		inReview.Response.Result = metav1.Status{
			Status:  "Failed",
			Message: fmt.Sprintf("Hook returned %d objects instead of %d", len(conversionResponse.ConvertedObjects), len(inReview.Request.Objects)),
		}
		return &inReview, nil
	}

	inReview.Response.Result = metav1.Status{
		Status: "Success",
	}

	inReview.Response.ConvertedObjects = conversionResponse.ConvertedObjects

	return &inReview, nil
}

// detectCrdName extracts crdName from the url path.
func detectCrdName(path string) string {
	return strings.TrimPrefix(path, "/")
}

func ExtractAPIVersions(objs []runtime.RawExtension) []string {
	verMap := make(map[string]struct{})
	res := make([]string, 0)

	for _, obj := range objs {
		fmt.Println("ASDASDASDASDASD--------")
		fmt.Println(obj)
		fmt.Println(obj.Object)
		fmt.Println(obj.Object.GetObjectKind())
		fmt.Println(obj.Object.GetObjectKind().GroupVersionKind())
		fmt.Println(obj.Object.GetObjectKind().GroupVersionKind().GroupVersion())
		fmt.Println("ASDASDASDASDASD--------- ++++++")
		version := obj.Object.GetObjectKind().GroupVersionKind().GroupVersion().String()

		if _, ok := verMap[version]; ok {
			continue
		}

		verMap[version] = struct{}{}
		res = append(res, version)
	}

	return res
}
