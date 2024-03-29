package context

import (
	"fmt"
	"reflect"
	"time"

	"github.com/flant/kube-client/fake"
	"github.com/flant/kube-client/manifest"
	kubeeventsmanager "github.com/flant/shell-operator/pkg/kube_events_manager"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
)

// if we use default, then we are not able to emulate global resources due to fake cluster limitations
// that's why an empty namespace is used here, specify namespace in a resource by your own if needed
var defaultNamespace = "" //nolint:gochecknoglobals

// StateController holds objects state for FakeCluster
type StateController struct {
	CurrentState map[string]manifest.Manifest
	fakeCluster  *fake.Cluster

	stopCh chan<- types.KubeEvent
}

// NewStateController creates controller to apply state changes
func NewStateController(fc *fake.Cluster, ev kubeeventsmanager.KubeEventsManager) *StateController {
	c := &StateController{
		CurrentState: make(map[string]manifest.Manifest),
		fakeCluster:  fc,
		stopCh:       ev.Ch(),
	}

	return c
}

func (c *StateController) SetInitialState(initialState string) error {
	manifests, err := manifest.ListFromYamlDocs(initialState)
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
func (c *StateController) ChangeState(newRawState string) error {
	newManifests, err := manifest.ListFromYamlDocs(newRawState)
	if err != nil {
		return fmt.Errorf("error while changing state: %v", err)
	}

	newState := make(map[string]manifest.Manifest)

	// Create new objects in FakeCluster
	for _, m := range newManifests {
		// save manifest to new State
		newState[m.Id()] = m

		currM, ok := c.CurrentState[m.Id()]
		if !ok {
			// Create object if not exist
			if err := c.fakeCluster.Create(m.Namespace(defaultNamespace), m); err != nil {
				return err
			}
			c.CurrentState[m.Id()] = m
		} else if !reflect.DeepEqual(currM, m) {
			// Update object
			if err := c.fakeCluster.Update(m.Namespace(defaultNamespace), m); err != nil {
				return err
			}
			c.CurrentState[m.Id()] = m
		}
	}
	// Remove obsolete objects
	for currID, currM := range c.CurrentState {
		if _, ok := newState[currID]; !ok {
			// Delete object
			if err := c.fakeCluster.Delete(currM.Namespace(defaultNamespace), currM); err != nil {
				return err
			}
			delete(c.CurrentState, currID)
		}
	}

	go func() {
		time.Sleep(1000 * time.Millisecond)
		c.stopCh <- types.KubeEvent{MonitorId: "STOP_EVENTS"}
	}()

	return nil
}
