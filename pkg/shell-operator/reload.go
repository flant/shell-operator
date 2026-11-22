// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package shell_operator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	pkg "github.com/flant/shell-operator/pkg"
	hook_types "github.com/flant/shell-operator/pkg/hook/types"
	"github.com/flant/shell-operator/pkg/webhook/admission"
	"github.com/flant/shell-operator/pkg/webhook/conversion"
)

const (
	enableKubernetesBindingsRetryTimeout = 2 * time.Minute
	enableKubernetesBindingsRetrySteps   = 30
)

// ReloadHooks re-discovers hooks from disk so that the operator picks up new or
// removed hook configurations without a process restart. It is safe for
// concurrent use: calls are serialised, because overlapping HookManager.Init() +
// Enable*Bindings sequences would operate on stale/overwritten hook indices.
//
// HookManager.Init() replaces the hook index atomically, but newly loaded
// hooks have uninitialised AdmissionLinks/ConversionLinks maps. We must
// call EnableAdmissionBindings / EnableConversionBindings on every hook that
// carries validating/mutating/conversion configs so that
// CanHandleAdmissionEvent / CanHandleConversionEvent can match incoming
// requests to the right hook.
//
// AdmissionWebhookManager.Init() and ConversionWebhookManager.Init() are
// deliberately NOT called here because they recreate HTTP servers and would
// either fail with "address already in use" or silently orphan the old
// listeners. As a consequence a webhook server that was never started (no
// validating/mutating hook existed at bootstrap) is not started by a reload
// either — callers that need the server up must have at least one such hook
// on disk before NewShellOperator.
func (op *ShellOperator) ReloadHooks(ctx context.Context) error {
	op.reloadMu.Lock()
	defer op.reloadMu.Unlock()

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled before reload: %w", err)
	}

	op.logger.Info("reloading hooks")

	if op.HookManager == nil {
		return fmt.Errorf("hook manager is not initialized")
	}

	// Snapshot the current registrations before any mutation. These are
	// compared against the rebuilt state later to determine which
	// ValidatingWebhookConfigurations / MutatingWebhookConfigurations / CRD
	// conversion settings are stale and must be unregistered.
	oldValidatingResources := make(map[string]*admission.ValidatingWebhookResource)
	oldMutatingResources := make(map[string]*admission.MutatingWebhookResource)
	oldConversionClientConfigs := make(map[string]*conversion.CrdClientConfig)

	if op.AdmissionWebhookManager != nil {
		for confID, resource := range op.AdmissionWebhookManager.ValidatingResources {
			oldValidatingResources[confID] = resource
		}
		for confID, resource := range op.AdmissionWebhookManager.MutatingResources {
			oldMutatingResources[confID] = resource
		}
	}

	if op.ConversionWebhookManager != nil {
		for crdName, cfg := range op.ConversionWebhookManager.ClientConfigs {
			oldConversionClientConfigs[crdName] = cfg
		}
	}

	// Re-discover hooks from disk. This is the step most likely to fail
	// (e.g. malformed hook file), so it MUST happen before we clear the
	// current registration maps — otherwise a failed Init leaves the
	// operator with empty maps and no way to unregister stale entries.
	if err := op.HookManager.Init(); err != nil {
		return fmt.Errorf("re-init hook manager: %w", err)
	}

	// Clear the current registration state AFTER Init has succeeded.
	// Rebuilding from scratch avoids stale webhook resources after hook
	// removals. The Enable*Bindings calls below repopulate these maps with
	// fresh entries for every discovered hook.
	if op.AdmissionWebhookManager != nil {
		op.AdmissionWebhookManager.ValidatingResources = make(map[string]*admission.ValidatingWebhookResource)
		op.AdmissionWebhookManager.MutatingResources = make(map[string]*admission.MutatingWebhookResource)
	}

	if op.ConversionWebhookManager != nil {
		op.ConversionWebhookManager.ClientConfigs = make(map[string]*conversion.CrdClientConfig)
	}

	// Enable admission bindings on every newly loaded hook so that
	// AdmissionLinks are populated and CanHandleEvent works.
	validatingHookNames, err := op.HookManager.GetHooksInOrder(hook_types.KubernetesValidating)
	if err != nil {
		return fmt.Errorf("get validating hooks: %w", err)
	}
	mutatingHookNames, err := op.HookManager.GetHooksInOrder(hook_types.KubernetesMutating)
	if err != nil {
		return fmt.Errorf("get mutating hooks: %w", err)
	}
	for _, name := range append(validatingHookNames, mutatingHookNames...) {
		if h := op.HookManager.GetHook(name); h != nil && h.HookController != nil {
			h.HookController.EnableAdmissionBindings()
		}
	}

	// Enable conversion bindings on every newly loaded hook so that
	// CanHandleConversionEvent works.
	conversionHookNames, err := op.HookManager.GetHooksInOrder(hook_types.KubernetesConversion)
	if err != nil {
		return fmt.Errorf("get conversion hooks: %w", err)
	}
	for _, name := range conversionHookNames {
		if h := op.HookManager.GetHook(name); h != nil && h.HookController != nil {
			h.HookController.EnableConversionBindings()
		}
	}

	// Re-enable kubernetes bindings on every newly loaded hook so that the
	// monitors that maintain object caches (snapshots) are recreated and
	// wired to the freshly rebuilt HookController.
	//
	// HookManager.Init() above replaces every Hook (and therefore every
	// HookController) with a brand new instance whose KubernetesController
	// has no registered monitors, and every kubernetes binding gets a fresh
	// random MonitorId. Admission/conversion hooks that pull data via
	// 'includeSnapshotsFrom' resolve their snapshot through the *current*
	// hook index, i.e. these freshly rebuilt controllers. If the monitors
	// for this new index are not wired, the 'snapshots' field is delivered
	// empty and validating webhooks decide on missing data (e.g. always
	// denying because a ModuleConfig snapshot looks absent).
	//
	// This MUST be driven to completion here rather than relying on the
	// caller to retry on error. The main queue enables bindings on the
	// *startup* hook index, but every ReloadHooks() call replaces that index
	// with new MonitorIds; the main queue's monitors are then orphaned
	// relative to the current index. If ReloadHooks() returned before wiring
	// the current index (e.g. a transient apiserver outage) and no further
	// reload were triggered, the current index would stay unwired and
	// snapshots empty indefinitely. So enableKubernetesBindings retries with
	// backoff until every hook is wired (or ctx/deadline ends).
	kubernetesHookNames, err := op.HookManager.GetHooksInOrder(hook_types.OnKubernetesEvent)
	if err != nil {
		return fmt.Errorf("get kubernetes hooks: %w", err)
	}
	if err := op.enableKubernetesBindings(ctx, kubernetesHookNames); err != nil {
		return err
	}

	if err := op.syncAdmissionWebhookConfigurations(ctx, oldValidatingResources, oldMutatingResources); err != nil {
		return fmt.Errorf("sync admission webhook configurations: %w", err)
	}

	if err := op.syncConversionWebhookConfigurations(ctx, oldConversionClientConfigs); err != nil {
		return fmt.Errorf("sync conversion webhook configurations: %w", err)
	}

	return nil
}

// enableKubernetesBindings wires the monitors (snapshot caches) for every
// kubernetes hook in the *current* hook index, retrying hooks that fail with
// bounded backoff until all succeed or the deadline/context ends.
func (op *ShellOperator) enableKubernetesBindings(ctx context.Context, hookNames []string) error {
	// Track hooks still needing a successful enable so each is wired exactly
	// once regardless of how many retry passes it takes.
	pending := make(map[string]struct{}, len(hookNames))
	for _, name := range hookNames {
		pending[name] = struct{}{}
	}

	// enableOne wires a single hook by name against the *current* hook index.
	// Use the same HookController entry point as the main queue, but pass nil
	// to avoid creating synchronization hook-run tasks during reload.
	enableOne := func(name string) error {
		h := op.HookManager.GetHook(name)
		if h == nil {
			return fmt.Errorf("hook %q not found in hook manager", name)
		}
		if h.HookController == nil {
			return fmt.Errorf("hook controller for hook %q is nil", name)
		}

		if err := h.HookController.HandleEnableKubernetesBindings(ctx, nil); err != nil {
			// Leave it pending and do NOT unlock events: the cache is not
			// filled, so emitting events would be premature.
			return err
		}

		// Cache is filled; allow the monitors to emit future events.
		h.HookController.UnlockKubernetesEvents()
		return nil
	}

	var lastErr error
	attempt := func() bool {
		var errs []error
		for name := range pending {
			if err := enableOne(name); err != nil {
				errs = append(errs, fmt.Errorf("enable kubernetes bindings for hook %q: %w", name, err))
				continue
			}
			delete(pending, name)
		}
		lastErr = errors.Join(errs...)
		return len(pending) == 0
	}

	// First pass: attempt every hook once. Continuing past a single failing
	// hook ensures one transient failure cannot starve the rest.
	if attempt() {
		return nil
	}

	op.logger.Warn("some kubernetes hooks failed to enable, retrying until wired",
		slog.Int("pending", len(pending)),
		log.Err(lastErr))

	// Retry only the still-pending hooks with capped exponential backoff until
	// they are all wired, the per-reload deadline is reached, or ctx is done.
	retryCtx, cancel := context.WithTimeout(ctx, enableKubernetesBindingsRetryTimeout)
	defer cancel()

	waitErr := wait.ExponentialBackoffWithContext(retryCtx, wait.Backoff{
		Duration: 500 * time.Millisecond,
		Factor:   2.0,
		Jitter:   0.1,
		Steps:    enableKubernetesBindingsRetrySteps,
		Cap:      10 * time.Second,
	}, func(context.Context) (bool, error) {
		return attempt(), nil
	})
	if waitErr != nil {
		// Deadline or context ended with hooks still unwired. Prefer the
		// underlying enable errors so the caller retries with the real cause;
		// fall back to the wait error if there is no enable error.
		if lastErr == nil {
			lastErr = waitErr
		}
		return fmt.Errorf("enable kubernetes bindings (%d hook(s) still unwired after %s): %w",
			len(pending), enableKubernetesBindingsRetryTimeout, lastErr)
	}

	op.logger.Info("all kubernetes hooks enabled and wired")
	return nil
}

// syncAdmissionWebhookConfigurations (re)registers the rebuilt
// Validating/MutatingWebhookConfigurations and removes the ones that no
// longer have a hook behind them.
func (op *ShellOperator) syncAdmissionWebhookConfigurations(
	ctx context.Context,
	oldValidatingResources map[string]*admission.ValidatingWebhookResource,
	oldMutatingResources map[string]*admission.MutatingWebhookResource,
) error {
	if op.AdmissionWebhookManager == nil {
		return nil
	}

	for confID, resource := range op.AdmissionWebhookManager.ValidatingResources {
		if err := resource.Register(ctx); err != nil {
			return fmt.Errorf("register validating webhook configuration %q: %w", confID, err)
		}
	}

	for confID, resource := range op.AdmissionWebhookManager.MutatingResources {
		if err := resource.Register(ctx); err != nil {
			return fmt.Errorf("register mutating webhook configuration %q: %w", confID, err)
		}
	}

	for confID, resource := range oldValidatingResources {
		if _, stillPresent := op.AdmissionWebhookManager.ValidatingResources[confID]; stillPresent {
			continue
		}

		if err := resource.Unregister(); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete stale validating webhook configuration %q: %w", confID, err)
		}
	}

	for confID, resource := range oldMutatingResources {
		if _, stillPresent := op.AdmissionWebhookManager.MutatingResources[confID]; stillPresent {
			continue
		}

		if err := resource.Unregister(); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete stale mutating webhook configuration %q: %w", confID, err)
		}
	}

	return nil
}

// syncConversionWebhookConfigurations points every CRD that still has a
// conversion hook at this operator's webhook and resets the conversion
// strategy of CRDs whose hook disappeared.
func (op *ShellOperator) syncConversionWebhookConfigurations(
	ctx context.Context,
	oldConversionClientConfigs map[string]*conversion.CrdClientConfig,
) error {
	if op.ConversionWebhookManager == nil {
		return nil
	}

	for crdName, cfg := range op.ConversionWebhookManager.ClientConfigs {
		if err := cfg.Update(ctx); err != nil {
			return fmt.Errorf("update conversion client config for crd %q: %w", crdName, err)
		}
	}

	for crdName := range oldConversionClientConfigs {
		if _, stillPresent := op.ConversionWebhookManager.ClientConfigs[crdName]; stillPresent {
			continue
		}

		if err := op.resetCRDConversionToNone(ctx, crdName); err != nil {
			return fmt.Errorf("cleanup stale conversion webhook config for crd %q: %w", crdName, err)
		}
	}

	return nil
}

// resetCRDConversionToNone switches a CRD back to the None conversion strategy
// so the apiserver stops calling a webhook that no longer has a hook behind it.
func (op *ShellOperator) resetCRDConversionToNone(ctx context.Context, crdName string) error {
	if op.KubeClient == nil {
		return fmt.Errorf("kubernetes client is not initialized")
	}

	crd, err := op.KubeClient.ApiExt().CustomResourceDefinitions().Get(ctx, crdName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("get CRD %q: %w", crdName, err)
	}

	if crd.Spec.Conversion == nil || crd.Spec.Conversion.Strategy != apiextensionsv1.WebhookConverter {
		return nil
	}

	crd.Spec.Conversion = &apiextensionsv1.CustomResourceConversion{
		Strategy: apiextensionsv1.NoneConverter,
	}

	if _, err := op.KubeClient.ApiExt().CustomResourceDefinitions().Update(ctx, crd, pkg.DefaultUpdateOptions()); err != nil {
		return fmt.Errorf("update CRD %q conversion strategy: %w", crdName, err)
	}

	return nil
}
