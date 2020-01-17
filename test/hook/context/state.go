package context

import (
	"fmt"
	"reflect"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
)

var defaultNamespace = "default"

// getUnstructuredName get name from unstructured object
func getUnstructuredName(unstructured map[string]interface{}) string {
	return unstructured["metadata"].(map[string]interface{})["name"].(string)
}

// getUnstructuredNamespace get namespace from unstructured object
func getUnstructuredNamespace(unstructured map[string]interface{}) string {
	namespace := unstructured["metadata"].(map[string]interface{})["namespace"]
	switch namespace.(type) {
	case string:
		return namespace.(string)
	default:
		return defaultNamespace
	}
}

// getGVRByKind get gvr by resource kind
func getGVRByKind(kind string) schema.GroupVersionResource {
	for _, group := range ClusterResources {
		for _, resource := range group.APIResources {
			if resource.Kind == kind {
				return schema.GroupVersionResource{
					Version:  resource.Version,
					Group:    resource.Group,
					Resource: resource.Name,
				}
			}
		}
	}
	return schema.GroupVersionResource{}
}

type State map[StateObject]unstructured.Unstructured

type StateObject struct {
	Kind      string
	name      string
	namespace string
}

type StateController struct {
	CurrentState State
}

// parseStateToObjects convert state from yaml to kubernetes objects
func parseStateToObjects(rawState string) (State, error) {
	state := make(State)

	sepYamlFiles := strings.Split(rawState, "---\n")
	for _, yamlFile := range sepYamlFiles {
		if strings.Trim(yamlFile, "\n") == "" {
			continue
		}

		decode := scheme.Codecs.UniversalDeserializer().Decode
		object, _, err := decode([]byte(yamlFile), nil, nil)
		if err != nil {
			return State{}, fmt.Errorf("decoding of initial state failed: %v", err)
		}

		unstructuredObject, err := runtime.DefaultUnstructuredConverter.ToUnstructured(object)
		if err != nil {
			return State{}, fmt.Errorf("decoding of initial state failed: %v", err)
		}

		state[StateObject{
			unstructuredObject["kind"].(string),
			getUnstructuredName(unstructuredObject),
			getUnstructuredNamespace(unstructuredObject),
		}] = unstructured.Unstructured{Object: unstructuredObject}
	}
	return state, nil
}

// NewStateController creates controller to apply state changes
func NewStateController(initialState string) (StateController, error) {
	state, err := parseStateToObjects(initialState)
	for stateObject, unstructuredObject := range state {
		err := createObject(stateObject, unstructuredObject)
		if err != nil {
			return StateController{}, fmt.Errorf("error while creating initial state: %v", err)
		}
	}
	return StateController{state}, err
}

func createObject(stateObject StateObject, unstructuredObject unstructured.Unstructured) error {
	gvr := getGVRByKind(stateObject.Kind)
	_, err := KubeClient.Dynamic().Resource(gvr).
		Namespace(stateObject.namespace).
		Create(&unstructuredObject, metav1.CreateOptions{}, []string{}...)
	if err != nil {
		return fmt.Errorf("creating object failed: %v", err)
	}
	return nil
}

func deleteObject(stateObject StateObject) error {
	gvr := getGVRByKind(stateObject.Kind)
	err := KubeClient.Dynamic().Resource(gvr).
		Namespace(stateObject.namespace).
		Delete(stateObject.name, &metav1.DeleteOptions{}, []string{}...)
	if err != nil {
		return fmt.Errorf("creating object failed: %v", err)
	}
	return nil
}

func updateObject(stateObject StateObject, unstructuredObject unstructured.Unstructured) error {
	gvr := getGVRByKind(stateObject.Kind)
	_, err := KubeClient.Dynamic().Resource(gvr).
		Namespace(stateObject.namespace).
		Update(&unstructuredObject, metav1.UpdateOptions{}, []string{}...)
	if err != nil {
		return fmt.Errorf("creating object failed: %v", err)
	}
	return nil
}

// ChangeState apply changes to current objects state
func (c *StateController) ChangeState(newRawState string) error {
	newState, err := parseStateToObjects(newRawState)
	if err != nil {
		return fmt.Errorf("error whiele changing state: %v", err)
	}

	for newStateObject, newUnstructuredObject := range newState {
		_, ok := c.CurrentState[newStateObject]
		if !ok {
			// Create object
			err := createObject(newStateObject, newUnstructuredObject)
			if err != nil {
				return err
			}
			c.CurrentState[newStateObject] = newUnstructuredObject
		}
	}
	for stateObject := range c.CurrentState {
		_, ok := newState[stateObject]
		if !ok {
			// Delete object
			err := deleteObject(stateObject)
			if err != nil {
				return err
			}
			delete(c.CurrentState, stateObject)
		}
	}
	for newStateObject, newUnstructuredObject := range newState {
		for stateObject, unstructuredObject := range c.CurrentState {
			if (stateObject == newStateObject) && !reflect.DeepEqual(unstructuredObject, newUnstructuredObject) {
				// Update object
				err := updateObject(newStateObject, newUnstructuredObject)
				if err != nil {
					return err
				}
				c.CurrentState[stateObject] = newUnstructuredObject
			}
		}
	}
	return nil
}
