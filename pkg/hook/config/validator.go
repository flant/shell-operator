package config

import (
	"encoding/json"
	"fmt"

	openapierrors "github.com/go-openapi/errors"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"
	"github.com/hashicorp/go-multierror"
)

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
		switch err := err.(type) {
		case *openapierrors.Validation:
			switch err.Code() {
			case openapierrors.RequiredFailCode:
				allErrs = multierror.Append(allErrs, err)

			case openapierrors.EnumFailCode:
				values := []string{}
				for _, allowedValue := range err.Values {
					if s, ok := allowedValue.(string); ok {
						values = append(values, s)
					} else {
						allowedJSON, _ := json.Marshal(allowedValue)
						values = append(values, string(allowedJSON))
					}
				}
				allErrs = multierror.Append(allErrs, err)
			default:
				allErrs = multierror.Append(allErrs, err)
			}
		default:
			allErrs = multierror.Append(allErrs, err)
		}
	}
	// NOTE: no validation errors, but config is not valid!
	if allErrs.Len() == 0 {
		allErrs = multierror.Append(allErrs, fmt.Errorf("configuration is not valid"))
	}
	return allErrs
}
