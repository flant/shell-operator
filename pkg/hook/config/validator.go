package config

import (
	"encoding/json"
	"strings"

	openapierrors "github.com/go-openapi/errors"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"
	"github.com/hashicorp/go-multierror"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type ValidateResult struct {
	IsValid bool
	Errors  field.ErrorList
}

func ValidateConfig(dataObj interface{}, schemaName string, rootName string) (isValid bool, multiErr error) {
	s := GetSchema(schemaName)
	if s == nil {
		return false, nil
	}

	validator := validate.NewSchemaValidator(s, nil, "", strfmt.Default) //, validate.DisableObjectArrayTypeCheck(true)

	result := validator.Validate(dataObj)
	if result.IsValid() {
		return true, nil
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
	return false, allErrs.ErrorOrNil()
}

func ValidateConfigVerbose(dataObj interface{}, schemaName string, rootName string) (isValid bool, multiErr error) {
	s := GetSchema(schemaName)
	if s == nil {
		return false, nil
	}

	validator := validate.NewSchemaValidator(s, nil, "", strfmt.Default) //, validate.DisableObjectArrayTypeCheck(true)

	result := validator.Validate(dataObj)
	if result.IsValid() {
		return true, nil
	}

	fldPath := field.NewPath(rootName)
	var allErrs *multierror.Error

	for _, err := range result.Errors {
		switch err := err.(type) {
		case *openapierrors.Validation:
			errPath := fldPath
			if len(err.Name) > 0 && err.Name != "." {
				errPath = errPath.Child(strings.TrimPrefix(err.Name, "."))
			}

			switch err.Code() {
			case openapierrors.RequiredFailCode:
				allErrs = multierror.Append(allErrs, field.Required(errPath, err.Error()))

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
				allErrs = multierror.Append(allErrs, field.NotSupported(errPath, err.Value, values))
			default:
				value := interface{}("")
				if err.Value != nil {
					value = err.Value
				}
				allErrs = multierror.Append(allErrs, field.Invalid(errPath, value, err.Error()))
			}
		default:
			allErrs = multierror.Append(allErrs, field.Invalid(fldPath, "", err.Error()))
		}
	}
	//
	//fmt.Printf("Errors: %+v\n", allErrs)
	return false, allErrs.ErrorOrNil()
}
