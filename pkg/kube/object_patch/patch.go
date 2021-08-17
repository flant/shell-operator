package object_patch

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

func (o *ObjectPatcher) ExecuteOperations(ops []Operation) error {
	log.Debug("Starting execute operations process")
	defer log.Debug("Finished execute operations process")

	var applyErrors = &multierror.Error{}
	for _, op := range ops {
		log.Debugf("Applying operation: %s", op.Description())

		// TODO remove after successful tests in deckhouse-oss
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

func (o *ObjectPatcher) ExecuteOperation(operation Operation) error {
	if operation == nil {
		return nil
	}

	switch v := operation.(type) {
	case *createOperation:
		return o.executeCreateOperation(v)
	case *deleteOperation:
		return o.executeDeleteOperation(v)
	case *patchOperation:
		return o.executePatchOperation(v)
	case *filterOperation:
		return o.executeFilterOperation(v)
	}

	return nil
}

func (o *ObjectPatcher) executeCreateOperation(op *createOperation) error {
	if op.object == nil {
		return fmt.Errorf("cannot create empty object")
	}

	// Convert object from interface{}.
	object, err := toUnstructured(op.object)
	if err != nil {
		return err
	}

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

// executePatchOperation applies a patch to the specified object using API call Patch.
//
// There 2 types of patches:
// - Merge — use Patch API call with MergePatchType.
// - JSON — use Patch API call with JSONPatchType.
//
// Other options:
// - WithSubresource — a subresource argument for Patch or Update API call.
// - IgnoreMissingObject — do not return error if the specified object is missing.
func (o *ObjectPatcher) executePatchOperation(op *patchOperation) error {
	if op.patchType == types.MergePatchType {
		log.Debug("Started MergePatchObject")
		defer log.Debug("Finished MergePatchObject")
	}
	if op.patchType == types.JSONPatchType {
		log.Debug("Started JSONPatchObject")
		defer log.Debug("Finished JSONPatchObject")

	}

	patchBytes, err := convertPatchToBytes(op.patch)
	if err != nil {
		return fmt.Errorf("encode %s patch for %s/%s/%s/%s: %v", op.patchType, op.apiVersion, op.kind, op.namespace, op.name, err)
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
func (o *ObjectPatcher) executeFilterOperation(op *filterOperation) error {
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

func (o *ObjectPatcher) executeDeleteOperation(op *deleteOperation) error {
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
