package context

import (
	"fmt"
	"reflect"

	"github.com/flant/shell-operator/pkg/kube/fake"
	"github.com/flant/shell-operator/pkg/utils/manifest"
)

// if we use default, then we are not able to emulate global resources due to fake cluster limitations
// that's why an empty namespace is used here, specify namespace in a resource by your own if needed
var defaultNamespace = ""

// StateController holds objects state for FakeCluster
type StateController struct {
	CurrentState map[string]manifest.Manifest

	fakeCluster *fake.FakeCluster
}

// NewStateController creates controller to apply state changes
func NewStateController(fc *fake.FakeCluster) *StateController {
	return &StateController{
		CurrentState: make(map[string]manifest.Manifest),
		fakeCluster:  fc,
	}
}

func (c *StateController) SetInitialState(initialState string) error {
	manifests, err := manifest.GetManifestListFromYamlDocuments(initialState)
	if err != nil {
		return fmt.Errorf("create initial state: %v", err)
	}

	newState := make(map[string]manifest.Manifest)

	for _, m := range manifests {
		err = c.fakeCluster.Create(m.Namespace(defaultNamespace), m)
		if err != nil {
			return fmt.Errorf("create initial state: %v", err)
		}
		newState[m.Id()] = m
	}

	c.CurrentState = newState

	return nil
}

// ChangeState apply changes to current objects state
func (c *StateController) ChangeState(newRawState string) (int, error) {
	newManifests, err := manifest.GetManifestListFromYamlDocuments(newRawState)
	if err != nil {
		return 0, fmt.Errorf("error while changing state: %v", err)
	}

	var newState = make(map[string]manifest.Manifest)

	generatedEvents := 0
	// Create new objects in FakeCluster
	for _, m := range newManifests {
		// save manifest to new State
		newState[m.Id()] = m

		currM, ok := c.CurrentState[m.Id()]
		if !ok {
			// Create object if not exist
			err = c.fakeCluster.Create(m.Namespace(defaultNamespace), m)
			//err := createObject(newStateObject, newUnstructuredObject)
			if err != nil {
				return generatedEvents, err
			}
			c.CurrentState[m.Id()] = m
			generatedEvents++
		} else {
			// Update object if changed
			if !reflect.DeepEqual(currM, m) {
				err := c.fakeCluster.Update(m.Namespace(defaultNamespace), m)
				if err != nil {
					return generatedEvents, err
				}
				c.CurrentState[m.Id()] = m
				generatedEvents++
			}
		}
	}
	// Remove obsolete objects
	for currId, currM := range c.CurrentState {
		if _, ok := newState[currId]; !ok {
			// Delete object
			err := c.fakeCluster.Delete(currM.Namespace(defaultNamespace), currM)
			if err != nil {
				return generatedEvents, err
			}
			delete(c.CurrentState, currId)
			generatedEvents++
		}
	}
	return generatedEvents, nil
}
