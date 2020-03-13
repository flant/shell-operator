package config

import (
	"fmt"

	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"
	"github.com/hashicorp/go-multierror"
)

// See https://github.com/kubernetes/apiextensions-apiserver/blob/1bb376f70aa2c6f2dec9a8c7f05384adbfac7fbb/pkg/apiserver/validation/validation.go#L47
func ValidateConfig(dataObj interface{}, s *spec.Schema, rootName string) (multiErr error) {
	if s == nil {
		return fmt.Errorf("validate config: schema is not provided")
	}

	validator := validate.NewSchemaValidator(s, nil, rootName, strfmt.Default) //, validate.DisableObjectArrayTypeCheck(true)

	result := validator.Validate(dataObj)
	if result.IsValid() {
		return nil
	}

	var allErrs *multierror.Error
	for _, err := range result.Errors {
		allErrs = multierror.Append(allErrs, err)
	}
	// NOTE: no validation errors, but config is not valid!
	if allErrs.Len() == 0 {
		allErrs = multierror.Append(allErrs, fmt.Errorf("configuration is not valid"))
	}
	return allErrs
}
