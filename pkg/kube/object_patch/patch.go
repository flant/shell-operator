package object_patch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

type ObjectPatcher struct {
	kubeClient KubeClient
	logger     *log.Entry
}

type KubeClient interface {
	kubernetes.Interface
	Dynamic() dynamic.Interface
	GroupVersionResource(apiVersion string, kind string) (schema.GroupVersionResource, error)
}

func NewObjectPatcher(kubeClient KubeClient) *ObjectPatcher {
	return &ObjectPatcher{
		kubeClient: kubeClient,
		logger:     log.WithField("operator.component", "KubernetesObjectPatcher"),
	}
}

func (o *ObjectPatcher) ExecuteOperations(ops []*Operation) error {
	log.Debug("Starting execute operations process")
	defer log.Debug("Finished execute operations process")

	var applyErrors = &multierror.Error{}
	for _, op := range ops {
		log.Debugf("Applying operation: %s", op.Description())

		// TODO remove after test in deckhouse-oss
		//if spec.Kind != "" && spec.ApiVersion == "" {
		//	res, err := o.kubeClient.APIResource(spec.ApiVersion, spec.Kind)
		//	if err != nil {
		//		return err
		//	}
		//	spec.ApiVersion = res.Group+"/"+res.Version
		//	spec.Kind = res.Kind
		//	log.Debugf("Applying spec resolve apiVersion: %s kind: %s", spec.ApiVersion, spec.Kind)
		//}

		if err := o.ExecuteOperation(op); err != nil {
			applyErrors = multierror.Append(applyErrors, err)
		}
	}

	return applyErrors.ErrorOrNil()
}

func (o *ObjectPatcher) ExecuteOperation(operation *Operation) error {
	if operation == nil {
		return nil
	}

	switch true {
	case operation.isCreate():
		return o.executeCreateOperation(operation)
	case operation.isDelete():
		return o.executeDeleteOperation(operation)
	case operation.isPatch():
		return o.executePatchOperation(operation)
	case operation.isDelete():
		return o.executeFilterOperation(operation)
	}

	return nil
}

// Delete uses apiVersion, kind, namespace and name to delete object from cluster.
//
// Options:
// - WithSubresource - delete a specified subresource
// - InForeground -  remove object when all dependants are removed (default)
// - InBackground - remove object immediately, dependants remove in background
// - NonCascading - remove object, dependants become orphan
//
// Missing object is ignored by default.
//func (o *ObjectPatcher) Delete(apiVersion, kind, namespace, name string, options ... PatcherOption) error {
//	log.Debug("Started Delete")
//	defer log.Debug("Finished Delete")
//
//	action := &patcherAction{
//		apiVersion: apiVersion,
//		kind: kind,
//		namespace: namespace,
//		name: name,
//		deletionPropagation: metav1.DeletePropagationForeground,
//	}
//
//	for _, option := range options {
//		option(action)
//	}
//
//	return o.deleteObject(action)
//}

//func (o *ObjectPatcher) CreateObject(object *unstructured.Unstructured, subresource string, options ... PatcherOption) error {
//	log.Debug("Started Create")
//	defer log.Debug("Finished Create")
//
//	if object == nil {
//		return fmt.Errorf("cannot create empty object")
//	}
//
//	apiVersion := object.GetAPIVersion()
//	kind := object.GetKind()
//
//	gvk, err := o.kubeClient.GroupVersionResource(apiVersion, kind)
//	if err != nil {
//		return err
//	}
//
//	log.Debug("Started Create API call")
//	_, err = o.kubeClient.Dynamic().Resource(gvk).Namespace(object.GetNamespace()).Create(context.TODO(), object, metav1.CreateOptions{}, generateSubresources(subresource)...)
//	log.Debug("Finished Create API call")
//
//	return err
//}
//
//func (o *ObjectPatcher) CreateObjectIfNotExists(object *unstructured.Unstructured, subresource string) error {
//	log.Debug("Started CreateObjectIfNotExists")
//	defer log.Debug("Finished CreateObjectIfNotExists")
//
//	if object == nil {
//		return fmt.Errorf("cannot create empty object")
//	}
//
//	apiVersion := object.GetAPIVersion()
//	kind := object.GetKind()
//
//	gvk, err := o.kubeClient.GroupVersionResource(apiVersion, kind)
//	if err != nil {
//		return err
//	}
//
//	log.Debug("Started Create API call")
//	_, err = o.kubeClient.Dynamic().Resource(gvk).Namespace(object.GetNamespace()).Create(context.TODO(), object, metav1.CreateOptions{}, generateSubresources(subresource)...)
//	log.Debug("Finished Create API call")
//
//	if errors.IsAlreadyExists(err) {
//		log.Debug("resource already exists, exiting without error")
//		return nil
//	}
//
//	return err
//}
//
//func (o *ObjectPatcher) CreateOrUpdateObject(object *unstructured.Unstructured, subresource string) error {
//	log.Debug("Started CreateOrUpdate")
//	defer log.Debug("Finished CreateOrUpdate")
//
//	if object == nil {
//		return fmt.Errorf("cannot create empty object")
//	}
//
//	apiVersion := object.GetAPIVersion()
//	kind := object.GetKind()
//
//	gvk, err := o.kubeClient.GroupVersionResource(apiVersion, kind)
//	if err != nil {
//		return err
//	}
//
//	log.Debug("Started Create API call")
//	_, err = o.kubeClient.Dynamic().Resource(gvk).Namespace(object.GetNamespace()).Create(context.TODO(), object, metav1.CreateOptions{}, generateSubresources(subresource)...)
//	log.Debug("Finished Create API call")
//
//	if errors.IsAlreadyExists(err) {
//		log.Debug("Object already exists, attempting to Update it with optimistic lock")
//
//		err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
//			log.Debug("Started Get API call")
//			existingObj, err := o.kubeClient.Dynamic().Resource(gvk).Namespace(object.GetNamespace()).Get(context.TODO(), object.GetName(), metav1.GetOptions{}, generateSubresources(subresource)...)
//			log.Debug("Finished Get API call")
//			if err != nil {
//				return err
//			}
//
//			objCopy := object.DeepCopy()
//			objCopy.SetResourceVersion(existingObj.GetResourceVersion())
//
//			log.Debug("Started Update API call")
//			_, err = o.kubeClient.Dynamic().Resource(gvk).Namespace(objCopy.GetNamespace()).Update(context.TODO(), objCopy, metav1.UpdateOptions{}, generateSubresources(subresource)...)
//			log.Debug("Finished Update API call")
//			return err
//		})
//	}
//
//	return err
//}

func (o *ObjectPatcher) executeCreateOperation(op *Operation) error {
	if op.object == nil {
		return fmt.Errorf("cannot create empty object")
	}

	// Convert object from interface{}.

	objectContent, err := runtime.DefaultUnstructuredConverter.ToUnstructured(op.object)
	if err != nil {
		return fmt.Errorf("convert to unstructured: %v", err)
	}
	object := &unstructured.Unstructured{Object: objectContent}

	apiVersion := object.GetAPIVersion()
	kind := object.GetKind()

	gvk, err := o.kubeClient.GroupVersionResource(apiVersion, kind)
	if err != nil {
		return err
	}

	log.Debug("Started Create API call")
	_, err = o.kubeClient.Dynamic().Resource(gvk).Namespace(object.GetNamespace()).Create(context.TODO(), object, metav1.CreateOptions{}, generateSubresources(op.subresource)...)
	log.Debug("Finished Create API call")

	if !op.ignoreIfExists && !op.updateIfExists {
		return err
	}

	objectExists := errors.IsAlreadyExists(err)

	if objectExists && op.ignoreIfExists {
		log.Debug("resource already exists, exiting without error")
		return nil
	}

	if objectExists && op.updateIfExists {
		log.Debug("Object already exists, attempting to Update it with optimistic lock")

		return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			log.Debug("Started Get API call")
			existingObj, err := o.kubeClient.Dynamic().Resource(gvk).Namespace(object.GetNamespace()).Get(context.TODO(), object.GetName(), metav1.GetOptions{}, generateSubresources(op.subresource)...)
			log.Debug("Finished Get API call")
			if err != nil {
				return err
			}

			objCopy := object.DeepCopy()
			objCopy.SetResourceVersion(existingObj.GetResourceVersion())

			log.Debug("Started Update API call")
			_, err = o.kubeClient.Dynamic().Resource(gvk).Namespace(objCopy.GetNamespace()).Update(context.TODO(), objCopy, metav1.UpdateOptions{}, generateSubresources(op.subresource)...)
			log.Debug("Finished Update API call")
			return err
		})
	}

	return nil
}

//// FilterObject
//// Deprecated: use Patch with options.
//func (o *ObjectPatcher) FilterObject(filterFunc func(*unstructured.Unstructured) (*unstructured.Unstructured, error),
//	apiVersion, kind, namespace, name, subresource string) error {
//
//	log.Debug("Started FilterObject")
//	defer log.Debug("Finished FilterObject")
//
//	return o.executeFilterOperation(NewFilterPatchOperation(filterFunc, apiVersion, kind, namespace, name,
//		WithSubresource(subresource),
//	))
//}

//// JQPatchObject
//// Deprecated: use Patch with options.
//func (o *ObjectPatcher) JQPatchObject(jqPatch, apiVersion, kind, namespace, name, subresource string) error {
//	log.Debug("Started JQPatchObject")
//	defer log.Debug("Finished JQPatchObject")
//
//	return o.Patch(apiVersion, kind, namespace, name,
//		WithSubresource(subresource),
//		UseJQPatch(jqPatch),
//	)
//}

// - UseJQPatch(jqFilter) — use Get and Update API calls to modify object with the jq expression.
// - UseFilterFunc(func) — use Get and Update API calls to modify object with the custom function.

// executePatchOperation applies a patch to the specified object using API call Patch.
//
// There 2 types of patches:
// - Merge — use Patch API call with MergePatchType.
// - JSON — use Patch API call with JSONPatchType.
//
// Other options:
// - WithSubresource — a subresource argument for Patch or Update API call.
// - IgnoreMissingObject — do not return error if the specified object is missing.
func (o *ObjectPatcher) executePatchOperation(op *Operation) error {
	if op.patchType == types.MergePatchType {
		log.Debug("Started MergePatchObject")
		defer log.Debug("Finished MergePatchObject")
	}
	if op.patchType == types.JSONPatchType {
		log.Debug("Started JSONPatchObject")
		defer log.Debug("Finished JSONPatchObject")

	}

	var err error

	// Convert patch data.
	var patchBytes []byte
	switch v := op.patch.(type) {
	case []byte:
		patchBytes = v
	case string:
		patchBytes = []byte(v)
	default:
		// try to encode to json
		patchBytes, err = json.Marshal(v)
		if err != nil {
			return fmt.Errorf("json encode %s patch for %s/%s/%s/%s: %v", op.patchType, op.apiVersion, op.kind, op.namespace, op.name, err)
		}
	}
	if patchBytes == nil {
		return fmt.Errorf("%s patch is nil for %s/%s/%s/%s", op.patchType, op.apiVersion, op.kind, op.namespace, op.name)
	}

	gvk, err := o.kubeClient.GroupVersionResource(op.apiVersion, op.kind)
	if err != nil {
		return err
	}

	log.Debug("Started Patch API call")
	_, err = o.kubeClient.Dynamic().Resource(gvk).Namespace(op.namespace).Patch(context.TODO(), op.name, op.patchType, patchBytes, metav1.PatchOptions{}, generateSubresources(op.subresource)...)
	log.Debug("Finished Patch API call")

	if op.ignoreMissingObject && errors.IsNotFound(err) {
		return nil
	}
	return err
}

// executeFilterOperation retrieves a specified object, modified it with
// filterFunc and calls update.
func (o *ObjectPatcher) executeFilterOperation(op *Operation) error {
	var err error

	if op.filterFunc == nil {
		return fmt.Errorf("FilterFunc is nil")
	}

	gvk, err := o.kubeClient.GroupVersionResource(op.apiVersion, op.kind)
	if err != nil {
		return err
	}

	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		log.Debug("Started Get API call")
		obj, err := o.kubeClient.Dynamic().Resource(gvk).Namespace(op.namespace).Get(context.TODO(), op.name, metav1.GetOptions{})
		log.Debug("Finished Get API call")
		if op.ignoreMissingObject && errors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}

		log.Debug("Started filtering object")
		filteredObj, err := op.filterFunc(obj)
		log.Debug("Finished filtering object")
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
		_, err = o.kubeClient.Dynamic().Resource(gvk).Namespace(op.namespace).Update(context.TODO(), filteredObj, metav1.UpdateOptions{}, generateSubresources(op.subresource)...)
		log.Debug("Finished Update API call")
		if err != nil {
			return err
		}

		return nil
	})

	return err
}

//// DeleteObject
//// Deprecated: use DeleteObject with options.
//func (o *ObjectPatcher) DeleteObject(apiVersion, kind, namespace, name, subresource string) error {
//	log.Debug("Started DeleteObject")
//	defer log.Debug("Finished DeleteObject")
//
//	return o.Delete(apiVersion, kind, namespace, name,
//		WithSubresource(subresource))
//}
//
//// DeleteObjectInBackground
//// Deprecated: use DeleteObject with options.
//func (o *ObjectPatcher) DeleteObjectInBackground(apiVersion, kind, namespace, name, subresource string) error {
//	log.Debug("Started DeleteObjectInBackground")
//	defer log.Debug("Finished DeleteObjectInBackground")
//
//	return o.Delete(apiVersion, kind, namespace, name,
//		WithSubresource(subresource),
//		InBackground())
//}
//
//// DeleteObjectNonCascading
//// Deprecated: use DeleteObject with options.
//func (o *ObjectPatcher) DeleteObjectNonCascading(apiVersion, kind, namespace, name, subresource string) error {
//	log.Debug("Started DeleteObjectNonCascading")
//	defer log.Debug("Finished DeleteObjectNonCascading")
//
//	return o.Delete(apiVersion, kind, namespace, name,
//		WithSubresource(subresource),
//		NonCascading())
//}

func (o *ObjectPatcher) executeDeleteOperation(op *Operation) error {
	gvk, err := o.kubeClient.GroupVersionResource(op.apiVersion, op.kind)
	if err != nil {
		return err
	}

	log.Debug("Started Delete API call")
	err = o.kubeClient.Dynamic().Resource(gvk).Namespace(op.namespace).Delete(context.TODO(), op.name, metav1.DeleteOptions{PropagationPolicy: &op.deletionPropagation}, op.subresource)
	log.Debug("Finished Delete API call")
	if errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	if op.deletionPropagation != metav1.DeletePropagationForeground {
		return nil
	}

	log.Debug("Waiting for object deletion")
	err = wait.Poll(time.Second, 20*time.Second, func() (done bool, err error) {
		log.Debug("Started Get API call")
		_, err = o.kubeClient.Dynamic().Resource(gvk).Namespace(op.namespace).Get(context.TODO(), op.name, metav1.GetOptions{})
		log.Debug("Finished Get API call")
		if errors.IsNotFound(err) {
			return true, nil
		}

		return false, err
	})

	return err
}
