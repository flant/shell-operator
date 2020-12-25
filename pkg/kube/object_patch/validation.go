package object_patch

import (
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func validateSpecs(specs []OperationSpec) error {
	validationErrs := field.ErrorList{}

	for _, spec := range specs {
		switch spec.Operation {
		case Create:
			validationErrs = append(validationErrs, validateCreateOpSpec(spec)...)
		case CreateOrUpdate:
			validationErrs = append(validationErrs, validateCreateOrUpdateOpSpec(spec)...)
		case Delete:
			validationErrs = append(validationErrs, validateDeleteOpSpec(spec)...)
		case DeleteInBackground:
			validationErrs = append(validationErrs, validateDeleteInBackgroundOpSpec(spec)...)
		case DeleteNonCascading:
			validationErrs = append(validationErrs, validateDeleteNonCascadingOpSpec(spec)...)
		case JQPatch:
			validationErrs = append(validationErrs, validateJQPatchOpSpec(spec)...)
		case MergePatch:
			validationErrs = append(validationErrs, validateMergePatchOpSpec(spec)...)
		case JSONPatch:
			validationErrs = append(validationErrs, validateJSONPatchOpSpec(spec)...)
		}
	}

	return validationErrs.ToAggregate()
}

func validateCreateOpSpec(spec OperationSpec) (errList field.ErrorList) {
	rootPath := field.NewPath("")

	if len(spec.Object) == 0 {
		errList = append(errList, field.Required(rootPath.Child("object"), ""))
	}

	return
}

func validateCreateOrUpdateOpSpec(spec OperationSpec) (errList field.ErrorList) {
	rootPath := field.NewPath("")

	if len(spec.Object) == 0 {
		errList = append(errList, field.Required(rootPath.Child("object"), ""))
	}

	return
}

func validateDeleteOpSpec(spec OperationSpec) (errList field.ErrorList) {
	rootPath := field.NewPath("")

	if len(spec.ApiVersion) == 0 {
		errList = append(errList, field.Required(rootPath.Child("apiVersion"), ""))
	}

	if len(spec.Kind) == 0 {
		errList = append(errList, field.Required(rootPath.Child("kind"), ""))
	}

	if len(spec.Name) == 0 {
		errList = append(errList, field.Required(rootPath.Child("name"), ""))
	}

	return
}

func validateDeleteInBackgroundOpSpec(spec OperationSpec) (errList field.ErrorList) {
	rootPath := field.NewPath("")

	if len(spec.ApiVersion) == 0 {
		errList = append(errList, field.Required(rootPath.Child("apiVersion"), ""))
	}

	if len(spec.Kind) == 0 {
		errList = append(errList, field.Required(rootPath.Child("kind"), ""))
	}

	if len(spec.Name) == 0 {
		errList = append(errList, field.Required(rootPath.Child("name"), ""))
	}

	return
}

func validateDeleteNonCascadingOpSpec(spec OperationSpec) (errList field.ErrorList) {
	rootPath := field.NewPath("")

	if len(spec.ApiVersion) == 0 {
		errList = append(errList, field.Required(rootPath.Child("apiVersion"), ""))
	}

	if len(spec.Kind) == 0 {
		errList = append(errList, field.Required(rootPath.Child("kind"), ""))
	}

	if len(spec.Name) == 0 {
		errList = append(errList, field.Required(rootPath.Child("name"), ""))
	}

	return
}

func validateJQPatchOpSpec(spec OperationSpec) (errList field.ErrorList) {
	rootPath := field.NewPath("")

	if len(spec.JQFilter) == 0 {
		errList = append(errList, field.Required(rootPath.Child("jqFilter"), ""))
	}

	if len(spec.ApiVersion) == 0 {
		errList = append(errList, field.Required(rootPath.Child("apiVersion"), ""))
	}

	if len(spec.Kind) == 0 {
		errList = append(errList, field.Required(rootPath.Child("kind"), ""))
	}

	if len(spec.Name) == 0 {
		errList = append(errList, field.Required(rootPath.Child("name"), ""))
	}

	return
}

func validateMergePatchOpSpec(spec OperationSpec) (errList field.ErrorList) {
	rootPath := field.NewPath("")

	if len(spec.MergePatch) == 0 {
		errList = append(errList, field.Required(rootPath.Child("mergePatch"), ""))
	}

	if len(spec.ApiVersion) == 0 {
		errList = append(errList, field.Required(rootPath.Child("apiVersion"), ""))
	}

	if len(spec.Kind) == 0 {
		errList = append(errList, field.Required(rootPath.Child("kind"), ""))
	}

	if len(spec.Name) == 0 {
		errList = append(errList, field.Required(rootPath.Child("name"), ""))
	}

	return
}

func validateJSONPatchOpSpec(spec OperationSpec) (errList field.ErrorList) {
	rootPath := field.NewPath("")

	if len(spec.JSONPatch) == 0 {
		errList = append(errList, field.Required(rootPath.Child("jsonPatch"), ""))
	}

	if len(spec.ApiVersion) == 0 {
		errList = append(errList, field.Required(rootPath.Child("apiVersion"), ""))
	}

	if len(spec.Kind) == 0 {
		errList = append(errList, field.Required(rootPath.Child("kind"), ""))
	}

	if len(spec.Name) == 0 {
		errList = append(errList, field.Required(rootPath.Child("name"), ""))
	}

	return
}
