package types

import (
	v1 "k8s.io/api/admission/v1"
)

type ValidatingEvent struct {
	WebhookId       string
	ConfigurationId string
	Review          *v1.AdmissionReview
}
