package object_patch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/hashicorp/go-multierror"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/jq"
	"github.com/flant/shell-operator/pkg/kube"
)

type ObjectPatcher struct {
	kubeClient kube.KubernetesClient
	logger     *log.Entry
}

func NewObjectPatcher(kubeClient kube.KubernetesClient) *ObjectPatcher {
	return &ObjectPatcher{
		kubeClient: kubeClient,
		logger:     log.WithField("operator.component", "KubernetesObjectPatcher"),
	}
}

func ParseSpecs(specBytes []byte) ([]OperationSpec, error) {
	log.Debugf("parsing patches:\n%s", specBytes)

	specs, err := unmarshalFromJSONOrYAML(specBytes)
	if err != nil {
		return nil, err
	}

	var validationErrors = &multierror.Error{}
	for _, spec := range specs {
		err = ValidateOperationSpec(spec, GetSchema("v0"), "")
		if err != nil {
			validationErrors = multierror.Append(validationErrors, err)
		}
	}

	return specs, validationErrors.ErrorOrNil()
}

func (o *ObjectPatcher) GenerateFromJSONAndExecuteOperations(specs []OperationSpec) error {
	log.Debug("Starting spec apply process")
	defer log.Debug("Finished spec apply process")

	var applyErrors = &multierror.Error{}
	for _, spec := range specs {
		log.Debugf("Applying spec: %s", spew.Sdump(spec))

		var operationError error

		switch spec.Operation {
		case Create:
			operationError = o.CreateObject(&unstructured.Unstructured{Object: spec.Object}, spec.Subresource)
		case CreateOrUpdate:
			operationError = o.CreateOrUpdateObject(&unstructured.Unstructured{Object: spec.Object}, spec.Subresource)
		case Delete:
			operationError = o.DeleteObject(spec.ApiVersion, spec.Kind, spec.Namespace, spec.Name, spec.Subresource)
		case DeleteInBackground:
			operationError = o.DeleteObjectInBackground(spec.ApiVersion, spec.Kind, spec.Namespace, spec.Name, spec.Subresource)
		case DeleteNonCascading:
			operationError = o.DeleteObjectNonCascading(spec.ApiVersion, spec.Kind, spec.Namespace, spec.Name, spec.Subresource)
		case JQPatch:
			operationError = o.JQPatchObject(spec.JQFilter, spec.ApiVersion, spec.Kind, spec.Namespace, spec.Name, spec.Subresource)
		case MergePatch:
			jsonMergePatch, err := json.Marshal(spec.MergePatch)
			if err != nil {
				applyErrors = multierror.Append(applyErrors, err)
				continue
			}

			operationError = o.MergePatchObject(jsonMergePatch, spec.ApiVersion, spec.Kind, spec.Namespace, spec.Name, spec.Subresource)
		case JSONPatch:
			jsonJsonPatch, err := json.Marshal(spec.JSONPatch)
			if err != nil {
				applyErrors = multierror.Append(applyErrors, err)
				continue
			}

			operationError = o.JSONPatchObject(jsonJsonPatch, spec.ApiVersion, spec.Kind, spec.Namespace, spec.Name, spec.Subresource)
		}

		if operationError != nil {
			applyErrors = multierror.Append(applyErrors, operationError)
		}
	}

	return applyErrors.ErrorOrNil()
}

func unmarshalFromJSONOrYAML(specs []byte) ([]OperationSpec, error) {
	fromJsonSpecs, err := unmarshalFromJson(specs)
	if err != nil {
		return unmarshalFromYaml(specs)
	}

	return fromJsonSpecs, nil
}

func unmarshalFromJson(jsonSpecs []byte) ([]OperationSpec, error) {
	var specSlice []OperationSpec

	dec := json.NewDecoder(bytes.NewReader(jsonSpecs))
	for {
		var doc OperationSpec
		err := dec.Decode(&doc)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		specSlice = append(specSlice, doc)
	}

	return specSlice, nil
}

func unmarshalFromYaml(yamlSpecs []byte) ([]OperationSpec, error) {
	var specSlice []OperationSpec

	dec := yaml.NewDecoder(bytes.NewReader(yamlSpecs))
	for {
		var doc OperationSpec
		err := dec.Decode(&doc)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		specSlice = append(specSlice, doc)
	}

	return specSlice, nil
}

func (o *ObjectPatcher) CreateObject(object *unstructured.Unstructured, subresource string) error {
	log.Debug("Started Create")
	defer log.Debug("Finished Create")

	if object == nil {
		return fmt.Errorf("cannot create empty object")
	}

	apiVersion := object.GetAPIVersion()
	kind := object.GetKind()

	gvk, err := o.kubeClient.GroupVersionResource(apiVersion, kind)
	if err != nil {
		return err
	}

	log.Debug("Started Create API call")
	_, err = o.kubeClient.Dynamic().Resource(gvk).Namespace(object.GetNamespace()).Create(object, metav1.CreateOptions{}, generateSubresources(subresource)...)
	log.Debug("Finished Create API call")

	return err
}

func (o *ObjectPatcher) CreateOrUpdateObject(object *unstructured.Unstructured, subresource string) error {
	log.Debug("Started CreateOrUpdate")
	defer log.Debug("Finished CreateOrUpdate")

	if object == nil {
		return fmt.Errorf("cannot create empty object")
	}

	apiVersion := object.GetAPIVersion()
	kind := object.GetKind()

	gvk, err := o.kubeClient.GroupVersionResource(apiVersion, kind)
	if err != nil {
		return err
	}

	log.Debug("Started Create API call")
	_, err = o.kubeClient.Dynamic().Resource(gvk).Namespace(object.GetNamespace()).Create(object, metav1.CreateOptions{}, generateSubresources(subresource)...)
	log.Debug("Finished Create API call")

	if errors.IsAlreadyExists(err) {
		log.Debug("Object already exists, attempting to Update it with optimistic lock")

		err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			log.Debug("Started Get API call")
			existingObj, err := o.kubeClient.Dynamic().Resource(gvk).Namespace(object.GetNamespace()).Get(object.GetName(), metav1.GetOptions{}, generateSubresources(subresource)...)
			log.Debug("Finished Get API call")
			if err != nil {
				return err
			}

			objCopy := object.DeepCopy()
			objCopy.SetResourceVersion(existingObj.GetResourceVersion())

			log.Debug("Started Update API call")
			_, err = o.kubeClient.Dynamic().Resource(gvk).Namespace(objCopy.GetNamespace()).Update(objCopy, metav1.UpdateOptions{}, generateSubresources(subresource)...)
			log.Debug("Finished Update API call")
			return err
		})
	}

	return err
}

func (o *ObjectPatcher) FilterObject(filterFunc func(*unstructured.Unstructured) (*unstructured.Unstructured, error),
	apiVersion, kind, namespace, name, subresource string) error {

	log.Debug("Started FilterObject")
	defer log.Debug("Finished FilterObject")

	if filterFunc == nil {
		return fmt.Errorf("FilterFunc is nil")
	}

	gvk, err := o.kubeClient.GroupVersionResource(apiVersion, kind)
	if err != nil {
		return err
	}

	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		log.Debug("Started Get API call")
		obj, err := o.kubeClient.Dynamic().Resource(gvk).Namespace(namespace).Get(name, metav1.GetOptions{})
		log.Debug("Finished Get API call")
		if err != nil {
			return err
		}

		log.Debug("Started filtering object via filterFunc")
		filteredObj, err := filterFunc(obj)
		log.Debug("Finished filtering object via filterFunc")
		if err != nil {
			return err
		}

		if equality.Semantic.DeepEqual(obj, filteredObj) {
			return nil
		}

		var filteredObjBuf bytes.Buffer
		err = unstructured.UnstructuredJSONScheme.Encode(filteredObj, &filteredObjBuf)
		if err != nil {
			return err
		}

		log.Debug("Started Update API call")
		_, err = o.kubeClient.Dynamic().Resource(gvk).Namespace(namespace).Update(filteredObj, metav1.UpdateOptions{}, generateSubresources(subresource)...)
		log.Debug("Finished Update API call")
		if err != nil {
			return err
		}

		return nil
	})

	return err
}

func (o *ObjectPatcher) JQPatchObject(jqPatch, apiVersion, kind, namespace, name, subresource string) error {
	log.Debug("Started JQPatchObject")
	defer log.Debug("Finished JQPatchObject")

	gvk, err := o.kubeClient.GroupVersionResource(apiVersion, kind)
	if err != nil {
		return err
	}

	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		log.Debug("Started Get API call")
		obj, err := o.kubeClient.Dynamic().Resource(gvk).Namespace(namespace).Get(name, metav1.GetOptions{})
		log.Debug("Finished Get API call")
		if err != nil {
			return err
		}

		log.Debug("Started applying jqPatch")
		patchedObj, err := applyJQPatch(jqPatch, obj)
		log.Debug("Finished applying jqPatch")
		if err != nil {
			return err
		}

		log.Debug("Started Update API call")
		_, err = o.kubeClient.Dynamic().Resource(gvk).Namespace(namespace).Update(patchedObj, metav1.UpdateOptions{}, generateSubresources(subresource)...)
		log.Debug("Finished Update API call")
		if err != nil {
			return err
		}

		return nil
	})

	return err
}

func (o *ObjectPatcher) MergePatchObject(mergePatch []byte, apiVersion, kind, namespace, name, subresource string) error {
	log.Debug("Started MergePatchObject")
	defer log.Debug("Finished MergePatchObject")

	log.Debug("Started Patch API call")
	gvk, err := o.kubeClient.GroupVersionResource(apiVersion, kind)
	log.Debug("Finished Patch API call")
	if err != nil {
		return err
	}

	_, err = o.kubeClient.Dynamic().Resource(gvk).Namespace(namespace).Patch(name, types.MergePatchType, mergePatch, metav1.PatchOptions{}, generateSubresources(subresource)...)

	return err
}

func (o *ObjectPatcher) JSONPatchObject(jsonPatch []byte, apiVersion, kind, namespace, name, subresource string) error {
	log.Debug("Started JSONPatchObject")
	defer log.Debug("Finished JSONPatchObject")

	gvk, err := o.kubeClient.GroupVersionResource(apiVersion, kind)
	if err != nil {
		return err
	}

	log.Debug("Started Patch API call")
	_, err = o.kubeClient.Dynamic().Resource(gvk).Namespace(namespace).Patch(name, types.JSONPatchType, jsonPatch, metav1.PatchOptions{}, generateSubresources(subresource)...)
	log.Debug("Finished Patch API call")

	return err
}

func (o *ObjectPatcher) DeleteObject(apiVersion, kind, namespace, name, subresource string) error {
	log.Debug("Started DeleteObject")
	defer log.Debug("Finished DeleteObject")

	return o.deleteObjectInternal(apiVersion, kind, namespace, name, subresource, metav1.DeletePropagationForeground)
}

func (o *ObjectPatcher) DeleteObjectInBackground(apiVersion, kind, namespace, name, subresource string) error {
	log.Debug("Started DeleteObjectInBackground")
	defer log.Debug("Finished DeleteObjectInBackground")

	return o.deleteObjectInternal(apiVersion, kind, namespace, name, subresource, metav1.DeletePropagationBackground)
}

func (o *ObjectPatcher) DeleteObjectNonCascading(apiVersion, kind, namespace, name, subresource string) error {
	log.Debug("Started DeleteObjectNonCascading")
	defer log.Debug("Finished DeleteObjectNonCascading")

	return o.deleteObjectInternal(apiVersion, kind, namespace, name, subresource, metav1.DeletePropagationOrphan)
}

func (o *ObjectPatcher) deleteObjectInternal(apiVersion, kind, namespace, name, subresource string, propagation metav1.DeletionPropagation) error {
	gvk, err := o.kubeClient.GroupVersionResource(apiVersion, kind)
	if err != nil {
		return err
	}

	log.Debug("Started Delete API call")
	err = o.kubeClient.Dynamic().Resource(gvk).Namespace(namespace).Delete(name, &metav1.DeleteOptions{PropagationPolicy: &propagation}, subresource)
	log.Debug("Finished Delete API call")
	if errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	if propagation != metav1.DeletePropagationForeground {
		return nil
	}

	log.Debug("Waiting for object deletion")
	err = wait.Poll(time.Second, 20*time.Second, func() (done bool, err error) {
		log.Debug("Started Get API call")
		_, err = o.kubeClient.Dynamic().Resource(gvk).Namespace(namespace).Get(name, metav1.GetOptions{})
		log.Debug("Finished Get API call")
		if errors.IsNotFound(err) {
			return true, nil
		}

		return false, err
	})

	return err
}

func applyJQPatch(jqFilter string, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	objBytes, err := obj.MarshalJSON()
	if err != nil {
		return nil, err
	}

	filterResult, err := jq.ApplyJqFilter(jqFilter, objBytes, app.JqLibraryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to apply jqFilter:\n%sto Object:\n%s\n"+
			"error: %s", jqFilter, obj, err)
	}

	var retObj = &unstructured.Unstructured{}
	_, _, err = unstructured.UnstructuredJSONScheme.Decode([]byte(filterResult), nil, retObj)
	if err != nil {
		return nil, fmt.Errorf("failed to convert filterResult:\n%s\nto Unstructured Object\nerror: %s", filterResult, err)
	}

	return retObj, nil
}

func generateSubresources(subresource string) (ret []string) {
	if subresource != "" {
		ret = append(ret, subresource)
	}

	return
}
