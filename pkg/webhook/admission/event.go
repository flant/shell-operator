package admission

import (
	v1 "k8s.io/api/admission/v1"
)

type Event struct {
	WebhookId       string
	ConfigurationId string
	Request         *v1.AdmissionRequest
}
