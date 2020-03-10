package context

import (
	"fmt"
	"reflect"

	"github.com/flant/shell-operator/pkg/utils/manifest"
)

var defaultNamespace = "default"

// StateController holds objects state for FakeCluster
type StateController struct {
	CurrentState map[string]manifest.Manifest
}

// NewStateController creates controller to apply state changes
func NewStateController() *StateController {
	return &StateController{
		CurrentState: make(map[string]manifest.Manifest),
	}
}

func (c *StateController) SetInitialState(initialState string) error {
	manifests, err := manifest.GetManifestListFromYamlDocuments(initialState)
	if err != nil {
		return fmt.Errorf("create initial state: %v", err)
	}

	newState := make(map[string]manifest.Manifest)

	for _, m := range manifests {
		err = FakeCluster.Create(defaultNamespace, m)
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
			err = FakeCluster.Create(defaultNamespace, m)
			//err := createObject(newStateObject, newUnstructuredObject)
			if err != nil {
				return generatedEvents, err
			}
			c.CurrentState[m.Id()] = m
			generatedEvents++
		} else {
			// Update object if changed
			if !reflect.DeepEqual(currM, m) {
				err := FakeCluster.Update(defaultNamespace, m)
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
			err := FakeCluster.Delete(defaultNamespace, currM)
			if err != nil {
				return generatedEvents, err
			}
			delete(c.CurrentState, currId)
			generatedEvents++
		}
	}
	return generatedEvents, nil
}
