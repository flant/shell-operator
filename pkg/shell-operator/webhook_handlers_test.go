package shell_operator

import (
	"context"
	"fmt"
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/flant/shell-operator/pkg/hook"
	"github.com/flant/shell-operator/pkg/hook/controller"
	htypes "github.com/flant/shell-operator/pkg/hook/types"
	kemtypes "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/flant/shell-operator/pkg/task"
	"github.com/flant/shell-operator/pkg/task/queue"
	"github.com/flant/shell-operator/pkg/webhook/admission"
	"github.com/flant/shell-operator/pkg/webhook/conversion"
)

// stubHookManager implements hook.HookManager for testing.
// Only the methods used by AdmissionEventHandler and ConversionEventHandler are meaningful.
type stubHookManager struct {
	detectAdmissionEventTypeFunc func(event admission.Event) htypes.BindingType
	handleAdmissionEventFunc     func(ctx context.Context, event admission.Event, createTaskFn func(*hook.Hook, controller.BindingExecutionInfo))
	findConversionChainFunc      func(crdName string, rule conversion.Rule) []conversion.Rule
	handleConversionEventFunc    func(ctx context.Context, crdName string, request *v1.ConversionRequest, rule conversion.Rule, createTaskFn func(*hook.Hook, controller.BindingExecutionInfo))
}

func (s *stubHookManager) Init() error                    { return nil }
func (s *stubHookManager) GetHook(name string) *hook.Hook { return nil }
func (s *stubHookManager) GetHookNames() []string         { return nil }
func (s *stubHookManager) GetHooksInOrder(htypes.BindingType) ([]string, error) {
	return nil, nil
}
func (s *stubHookManager) CreateTasksFromKubeEvent(_ kemtypes.KubeEvent, _ func(*hook.Hook, controller.BindingExecutionInfo) task.Task) []task.Task {
	return nil
}
func (s *stubHookManager) HandleCreateTasksFromScheduleEvent(_ string, _ func(*hook.Hook, controller.BindingExecutionInfo) task.Task) []task.Task {
	return nil
}
func (s *stubHookManager) HandleAdmissionEvent(ctx context.Context, event admission.Event, createTaskFn func(*hook.Hook, controller.BindingExecutionInfo)) {
	if s.handleAdmissionEventFunc != nil {
		s.handleAdmissionEventFunc(ctx, event, createTaskFn)
	}
}
func (s *stubHookManager) DetectAdmissionEventType(event admission.Event) htypes.BindingType {
	if s.detectAdmissionEventTypeFunc != nil {
		return s.detectAdmissionEventTypeFunc(event)
	}
	return htypes.KubernetesValidating
}
func (s *stubHookManager) HandleConversionEvent(ctx context.Context, crdName string, request *v1.ConversionRequest, rule conversion.Rule, createTaskFn func(*hook.Hook, controller.BindingExecutionInfo)) {
	if s.handleConversionEventFunc != nil {
		s.handleConversionEventFunc(ctx, crdName, request, rule, createTaskFn)
	}
}
func (s *stubHookManager) FindConversionChain(crdName string, rule conversion.Rule) []conversion.Rule {
	if s.findConversionChainFunc != nil {
		return s.findConversionChainFunc(crdName, rule)
	}
	return nil
}

// makeMinimalHook returns a hook with just the Name set, sufficient for factory calls.
func makeMinimalHook(name string) *hook.Hook {
	return hook.NewHook(name, "/stub/path/"+name, false, false, "", log.NewNop())
}

// stubTaskRunner is a TaskRunner that sets an admissionResponse or conversionResponse prop
// and then returns Success.
type stubTaskRunnerWithProp struct {
	propKey   string
	propValue interface{}
	status    queue.TaskStatus
}

func (s *stubTaskRunnerWithProp) run(_ context.Context, t task.Task) queue.TaskResult {
	if s.propKey != "" {
		t.SetProp(s.propKey, s.propValue)
	}
	return queue.TaskResult{Status: s.status}
}

// ---- AdmissionEventHandler tests ----

func TestAdmissionEventHandler_Handle_noHookFound_returnsError(t *testing.T) {
	hm := &stubHookManager{
		handleAdmissionEventFunc: func(_ context.Context, _ admission.Event, _ func(*hook.Hook, controller.BindingExecutionInfo)) {
			// deliberate no-op: does not call createTaskFn → admissionTask stays nil
		},
	}
	runner := func(_ context.Context, _ task.Task) queue.TaskResult {
		return queue.TaskResult{Status: queue.Success}
	}
	h := NewAdmissionEventHandler(hm, runner, log.NewNop())
	event := admission.Event{
		WebhookId:       "wh1",
		ConfigurationId: "cfg1",
		Request:         &admissionv1.AdmissionRequest{},
	}

	resp, err := h.Handle(context.Background(), event)
	assert.Nil(t, resp)
	require.Error(t, err)
}

func TestAdmissionEventHandler_Handle_taskRunnerFail_returnsDenied(t *testing.T) {
	hm := &stubHookManager{
		handleAdmissionEventFunc: func(_ context.Context, _ admission.Event, fn func(*hook.Hook, controller.BindingExecutionInfo)) {
			fn(makeMinimalHook("test-hook"), controller.BindingExecutionInfo{})
		},
	}
	runner := func(_ context.Context, _ task.Task) queue.TaskResult {
		return queue.TaskResult{Status: queue.Fail}
	}
	h := NewAdmissionEventHandler(hm, runner, log.NewNop())
	event := admission.Event{
		WebhookId:       "wh1",
		ConfigurationId: "cfg1",
		Request:         &admissionv1.AdmissionRequest{},
	}

	resp, err := h.Handle(context.Background(), event)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.False(t, resp.Allowed)
	assert.Equal(t, "Hook failed", resp.Message)
}

func TestAdmissionEventHandler_Handle_taskRunnerSuccess_returnsProp(t *testing.T) {
	want := &admission.Response{Allowed: true, Message: "ok"}
	hm := &stubHookManager{
		handleAdmissionEventFunc: func(_ context.Context, _ admission.Event, fn func(*hook.Hook, controller.BindingExecutionInfo)) {
			fn(makeMinimalHook("test-hook"), controller.BindingExecutionInfo{})
		},
	}
	runner := (&stubTaskRunnerWithProp{propKey: "admissionResponse", propValue: want, status: queue.Success}).run
	h := NewAdmissionEventHandler(hm, runner, log.NewNop())
	event := admission.Event{
		WebhookId:       "wh1",
		ConfigurationId: "cfg1",
		Request:         &admissionv1.AdmissionRequest{},
	}

	resp, err := h.Handle(context.Background(), event)
	require.NoError(t, err)
	assert.Equal(t, want, resp)
}

func TestAdmissionEventHandler_Handle_badPropType_returnsError(t *testing.T) {
	hm := &stubHookManager{
		handleAdmissionEventFunc: func(_ context.Context, _ admission.Event, fn func(*hook.Hook, controller.BindingExecutionInfo)) {
			fn(makeMinimalHook("test-hook"), controller.BindingExecutionInfo{})
		},
	}
	runner := (&stubTaskRunnerWithProp{propKey: "admissionResponse", propValue: "wrong-type", status: queue.Success}).run
	h := NewAdmissionEventHandler(hm, runner, log.NewNop())
	event := admission.Event{
		WebhookId:       "wh1",
		ConfigurationId: "cfg1",
		Request:         &admissionv1.AdmissionRequest{},
	}

	resp, err := h.Handle(context.Background(), event)
	assert.Nil(t, resp)
	require.Error(t, err)
}

// ---- ConversionEventHandler tests ----

func TestConversionEventHandler_Handle_noConversionPath_returnsNilError(t *testing.T) {
	hm := &stubHookManager{
		findConversionChainFunc: func(_ string, _ conversion.Rule) []conversion.Rule {
			return nil // empty → no path found
		},
	}
	runner := func(_ context.Context, _ task.Task) queue.TaskResult {
		return queue.TaskResult{Status: queue.Success}
	}
	h := NewConversionEventHandler(hm, runner, log.NewNop())

	req := &v1.ConversionRequest{
		DesiredAPIVersion: "v2",
		Objects:           []runtime.RawExtension{{Raw: []byte(`{"apiVersion":"v1","kind":"Foo"}`)}},
	}
	resp, err := h.Handle(context.Background(), "foo.example.com", req)
	// No chain found → done == false → returns success response with original objects
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestConversionEventHandler_Handle_taskRunnerFail_returnsFailedResponse(t *testing.T) {
	convRule := conversion.Rule{FromVersion: "v1", ToVersion: "v2"}
	hm := &stubHookManager{
		findConversionChainFunc: func(_ string, _ conversion.Rule) []conversion.Rule {
			return []conversion.Rule{convRule}
		},
		handleConversionEventFunc: func(_ context.Context, _ string, _ *v1.ConversionRequest, _ conversion.Rule, fn func(*hook.Hook, controller.BindingExecutionInfo)) {
			fn(makeMinimalHook("conv-hook"), controller.BindingExecutionInfo{})
		},
	}
	runner := func(_ context.Context, _ task.Task) queue.TaskResult {
		return queue.TaskResult{Status: queue.Fail}
	}
	h := NewConversionEventHandler(hm, runner, log.NewNop())

	req := &v1.ConversionRequest{
		DesiredAPIVersion: "v2",
		Objects:           []runtime.RawExtension{{Raw: []byte(`{"apiVersion":"v1","kind":"Foo"}`)}},
	}
	resp, err := h.Handle(context.Background(), "foo.example.com", req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.FailedMessage)
	assert.Nil(t, resp.ConvertedObjects)
}

func TestConversionEventHandler_Handle_noHookForPath_returnsError(t *testing.T) {
	convRule := conversion.Rule{FromVersion: "v1", ToVersion: "v2"}
	hm := &stubHookManager{
		findConversionChainFunc: func(_ string, _ conversion.Rule) []conversion.Rule {
			return []conversion.Rule{convRule}
		},
		handleConversionEventFunc: func(_ context.Context, _ string, _ *v1.ConversionRequest, _ conversion.Rule, _ func(*hook.Hook, controller.BindingExecutionInfo)) {
			// no-op: does not call createTaskFn → convTask stays nil
		},
	}
	runner := func(_ context.Context, _ task.Task) queue.TaskResult {
		return queue.TaskResult{Status: queue.Success}
	}
	h := NewConversionEventHandler(hm, runner, log.NewNop())

	req := &v1.ConversionRequest{
		DesiredAPIVersion: "v2",
		Objects:           []runtime.RawExtension{{Raw: []byte(`{"apiVersion":"v1","kind":"Foo"}`)}},
	}
	_, err := h.Handle(context.Background(), "foo.example.com", req)
	require.Error(t, err)
}

func TestConversionEventHandler_Handle_badPropType_returnsError(t *testing.T) {
	convRule := conversion.Rule{FromVersion: "v1", ToVersion: "v2"}
	hm := &stubHookManager{
		findConversionChainFunc: func(_ string, _ conversion.Rule) []conversion.Rule {
			return []conversion.Rule{convRule}
		},
		handleConversionEventFunc: func(_ context.Context, _ string, _ *v1.ConversionRequest, _ conversion.Rule, fn func(*hook.Hook, controller.BindingExecutionInfo)) {
			fn(makeMinimalHook("conv-hook"), controller.BindingExecutionInfo{})
		},
	}
	runner := (&stubTaskRunnerWithProp{propKey: "conversionResponse", propValue: "wrong-type", status: queue.Success}).run
	h := NewConversionEventHandler(hm, runner, log.NewNop())

	req := &v1.ConversionRequest{
		DesiredAPIVersion: "v2",
		Objects:           []runtime.RawExtension{{Raw: []byte(`{"apiVersion":"v1","kind":"Foo"}`)}},
	}
	_, err := h.Handle(context.Background(), "foo.example.com", req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hook task prop error")
}

func TestConversionEventHandler_Handle_success_returnsProp(t *testing.T) {
	convRule := conversion.Rule{FromVersion: "v1", ToVersion: "v2"}
	want := &conversion.Response{
		ConvertedObjects: []runtime.RawExtension{{Raw: []byte(`{"apiVersion":"v2","kind":"Foo"}`)}},
	}
	hm := &stubHookManager{
		findConversionChainFunc: func(_ string, _ conversion.Rule) []conversion.Rule {
			return []conversion.Rule{convRule}
		},
		handleConversionEventFunc: func(_ context.Context, _ string, _ *v1.ConversionRequest, _ conversion.Rule, fn func(*hook.Hook, controller.BindingExecutionInfo)) {
			fn(makeMinimalHook("conv-hook"), controller.BindingExecutionInfo{})
		},
	}
	runner := func(_ context.Context, t task.Task) queue.TaskResult {
		t.SetProp("conversionResponse", want)
		return queue.TaskResult{Status: queue.Success}
	}
	h := NewConversionEventHandler(hm, runner, log.NewNop())

	req := &v1.ConversionRequest{
		DesiredAPIVersion: "v2",
		Objects:           []runtime.RawExtension{{Raw: []byte(`{"apiVersion":"v1","kind":"Foo"}`)}},
	}
	resp, err := h.Handle(context.Background(), "foo.example.com", req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, want.ConvertedObjects, resp.ConvertedObjects)
}

// compile-time check that stubHookManager satisfies the interface
var _ hook.HookManager = (*stubHookManager)(nil)

// Ensure fmt is used to avoid unused import errors.
var _ = fmt.Sprintf
