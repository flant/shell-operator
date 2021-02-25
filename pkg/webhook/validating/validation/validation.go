package validation

import (
	"github.com/hashicorp/go-multierror"
	v1 "k8s.io/api/admissionregistration/v1"
	genericvalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

/*
An adaption of validation code from kubernetes/kubernetes repo:
https://github.com/kubernetes/kubernetes/blob/v1.20.1/pkg/apis/admissionregistration/validation/validation.go

Supports only "v1" version.
*/

// ValidateValidatingWebhookConfiguration validates a webhook before creation.
func ValidateValidatingWebhookConfiguration(e *v1.ValidatingWebhookConfiguration) error {
	var allErrors *multierror.Error
	metaErrors := genericvalidation.ValidateObjectMeta(&e.ObjectMeta, false, genericvalidation.NameIsDNSSubdomain, field.NewPath("metadata"))
	allErrors = AppendFieldList(allErrors, metaErrors)

	allErrors = multierror.Append(allErrors, ValidateValidatingWebhooks(e))

	return allErrors.ErrorOrNil()
}

func ValidateValidatingWebhooks(e *v1.ValidatingWebhookConfiguration) error {
	var allErrors *multierror.Error

	hookNames := make(map[string]struct{})
	for i, hook := range e.Webhooks {
		allErrors = multierror.Append(allErrors, ValidateValidatingWebhook(&hook, field.NewPath("webhooks").Index(i)))
		//allErrors = append(allErrors, validateAdmissionReviewVersions(hook.AdmissionReviewVersions, opts.requireRecognizedAdmissionReviewVersion, field.NewPath("webhooks").Index(i).Child("admissionReviewVersions"))...)
		if len(hook.Name) > 0 {
			if _, has := hookNames[hook.Name]; has {
				allErrors = multierror.Append(allErrors, field.Duplicate(field.NewPath("webhooks").Index(i).Child("name"), hook.Name))
			}
			hookNames[hook.Name] = struct{}{}
		}
	}

	return allErrors.ErrorOrNil()
}

// ValidateValidatingWebhook checks a "webhook" section.
//
// "failurePolicy" and "sideEffect" are validated by hook config schema.
//
// "clientConfig" and "matchPolicy" are filled in WebhookResource, no need to check.
//
// "rules", "timeoutSeconds", "namespaceSelector", "objectSelector" are from hook config.
func ValidateValidatingWebhook(hook *v1.ValidatingWebhook, fldPath *field.Path) error {
	var allErrors *multierror.Error
	// hook.Name must be fully qualified
	allErrors = AppendFieldList(allErrors, utilvalidation.IsFullyQualifiedName(fldPath.Child("name"), hook.Name))

	for i, rule := range hook.Rules {
		allErrors = AppendFieldList(allErrors, ValidateRuleWithOperations(&rule, fldPath.Child("rules").Index(i)))
	}

	if hook.TimeoutSeconds != nil && (*hook.TimeoutSeconds > 30 || *hook.TimeoutSeconds < 1) {
		allErrors = multierror.Append(allErrors, field.Invalid(fldPath.Child("timeoutSeconds"), *hook.TimeoutSeconds, "the timeout value must be between 1 and 30 seconds"))
	}

	if hook.NamespaceSelector != nil {
		allErrors = AppendFieldList(allErrors, metav1validation.ValidateLabelSelector(hook.NamespaceSelector, fldPath.Child("namespaceSelector")))
	}

	if hook.ObjectSelector != nil {
		allErrors = AppendFieldList(allErrors, metav1validation.ValidateLabelSelector(hook.ObjectSelector, fldPath.Child("objectSelector")))
	}

	//cc := hook.ClientConfig
	//switch {
	//case (cc.URL == nil) == (cc.Service == nil):
	//	allErrors = multierror.Append(allErrors, field.Required(fldPath.Child("clientConfig"), "exactly one of url or service is required"))
	//case cc.URL != nil:
	//	allErrors = multierror.Append(allErrors, webhook.ValidateWebhookURL(fldPath.Child("clientConfig").Child("url"), *cc.URL, true)...)
	//case cc.Service != nil:
	//	allErrors = multierror.Append(allErrors, webhook.ValidateWebhookService(fldPath.Child("clientConfig").Child("service"), cc.Service.Name, cc.Service.Namespace, cc.Service.Path, cc.Service.Port)...)
	//}
	return allErrors
}

func AppendFieldList(err error, list field.ErrorList) *multierror.Error {
	var res *multierror.Error
	res = multierror.Append(res, err)
	for _, er := range list {
		res = multierror.Append(res, er)
	}
	return res
}
