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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Error reading request body"))
		log.Errorf("Error reading request body: %v", err)
		return
	}

	conversionResponse, err := h.handleReviewRequest(r.URL.Path, bodyBytes)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
		return
	}

	respBytes, err := json.Marshal(conversionResponse)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Error json encoding ConversionReview"))
		log.Errorf("Error json encoding ConversionReview: %v", err)
		return
	}

	w.Header().Set("Content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(respBytes)
}

// See https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/#write-a-conversion-webhook-server
// This code always response with v1 ConversionReview: it works for 1.16+.
func (h *WebhookHandler) handleReviewRequest(path string, body []byte) (*v1.ConversionReview, error) {
	crdName := detectCrdName(path)
	log.Infof("Got ConversionReview request for crd/%s", crdName)

	var inReview v1.ConversionReview
	err := json.Unmarshal(body, &inReview)
	if err != nil {
		log.Errorf("Error parsing ConversionReview: %v", err)
		return nil, fmt.Errorf("fail to parse ConversionReview")
	}

	review := &v1.ConversionReview{
		TypeMeta: inReview.TypeMeta,
		Response: &v1.ConversionResponse{
			UID: inReview.Request.UID,
		},
	}

	if h.Manager.EventHandlerFn == nil {
		review.Response.Result = metav1.Status{
			Status:  "Failed",
			Message: "ConversionReview handler is not defined",
		}
		return review, nil
	}

	event, err := prepareConversionEvent(crdName, &inReview)
	if err != nil {
		return nil, err
	}

	conversionResponse, err := h.Manager.EventHandlerFn(event)
	if err != nil {
		review.Response.Result = metav1.Status{
			Status:  "Failed",
			Message: err.Error(),
		}
		return review, nil
	}

	if conversionResponse.FailedMessage != "" {
		review.Response.Result = metav1.Status{
			Status:  "Failed",
			Message: conversionResponse.FailedMessage,
		}
		return review, nil
	}

	if len(inReview.Request.Objects) != len(conversionResponse.ConvertedObjects) {
		review.Response.Result = metav1.Status{
			Status:  "Failed",
			Message: fmt.Sprintf("Hook returned %d objects instead of %d", len(conversionResponse.ConvertedObjects), len(review.Request.Objects)),
		}
		return review, nil
	}

	review.Response.Result = metav1.Status{
		Status: "Success",
	}

	// Convert objects from hook into to array of runtime.RawExtension
	rawObjects := make([]runtime.RawExtension, len(conversionResponse.ConvertedObjects))
	for i, obj := range conversionResponse.ConvertedObjects {
		tmpObj := obj
		rawObjects[i] = runtime.RawExtension{Object: &tmpObj}
	}
	review.Response.ConvertedObjects = rawObjects

	return review, nil
}

// detectCrdName extracts crdName from the url path.
func detectCrdName(path string) string {
	return strings.TrimPrefix(path, "/")
}

func prepareConversionEvent(crdName string, review *v1.ConversionReview) (event Event, err error) {
	event.CrdName = crdName
	event.Review = review
	event.Objects, err = rawExtensionToUnstructured(review.Request.Objects)
	return event, err
}

func ExtractAPIVersions(objs []unstructured.Unstructured) []string {
	verMap := map[string]bool{}
	for _, obj := range objs {
		verMap[obj.GetAPIVersion()] = true
	}
	res := make([]string, 0)
	for ver := range verMap {
		res = append(res, ver)
	}
	return res
}

func rawExtensionToUnstructured(objects []runtime.RawExtension) ([]unstructured.Unstructured, error) {
	res := make([]unstructured.Unstructured, 0)

	for _, obj := range objects {
		cr := unstructured.Unstructured{}

		if err := cr.UnmarshalJSON(obj.Raw); err != nil {
			return nil, fmt.Errorf("failed to unmarshall object in conversion request with error: %v", err)
		}

		res = append(res, cr)
	}

	return res, nil
}
