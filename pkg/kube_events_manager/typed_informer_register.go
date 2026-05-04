package kubeeventsmanager

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// init registers every GVR that client-go ships a typed SharedInformerFactory
// informer for (client-go v0.33.x). When --use-typed-informers is enabled and
// the operator's monitor targets one of these GVRs, the FactoryStore will use
// informers.SharedInformerFactory (typed structs) instead of
// dynamicinformer.DynamicSharedInformerFactory (*unstructured.Unstructured).
//
// Coverage rules:
//   - All stable (v1) kinds are included unconditionally.
//   - Beta/alpha kinds are included because the fallback in factory.go is
//     safe: if ForResource returns an error the factory silently falls back to
//     the dynamic informer, so listing a GVR here that is absent from the
//     cluster is harmless.
//   - extensions/v1beta1 is intentionally omitted — those resources were
//     removed in Kubernetes 1.16 and their stable equivalents (apps/v1,
//     networking.k8s.io/v1) are already covered.
//   - CRDs can never appear here; they have no pre-compiled typed struct in
//     client-go and must always use the dynamic informer.
func init() {
	register := func(group, version, resource string) {
		registerTypedKind(schema.GroupVersionResource{
			Group:    group,
			Version:  version,
			Resource: resource,
		})
	}

	// core/v1
	register("", "v1", "pods")
	register("", "v1", "configmaps")
	register("", "v1", "secrets")
	register("", "v1", "services")
	register("", "v1", "endpoints")
	register("", "v1", "namespaces")
	register("", "v1", "nodes")
	register("", "v1", "events")
	register("", "v1", "serviceaccounts")
	register("", "v1", "persistentvolumes")
	register("", "v1", "persistentvolumeclaims")
	register("", "v1", "replicationcontrollers")
	register("", "v1", "resourcequotas")
	register("", "v1", "limitranges")
	register("", "v1", "podtemplates")
	register("", "v1", "componentstatuses")

	// apps/v1
	register("apps", "v1", "deployments")
	register("apps", "v1", "statefulsets")
	register("apps", "v1", "daemonsets")
	register("apps", "v1", "replicasets")

	// batch/v1
	register("batch", "v1", "jobs")
	register("batch", "v1", "cronjobs")

	// autoscaling — v1 and v2 (v2beta1/v2beta2 are deprecated, omitted)
	register("autoscaling", "v1", "horizontalpodautoscalers")
	register("autoscaling", "v2", "horizontalpodautoscalers")

	// networking.k8s.io/v1
	register("networking.k8s.io", "v1", "ingresses")
	register("networking.k8s.io", "v1", "networkpolicies")

	// coordination.k8s.io — Leases are high-volume: one per node for kubelet
	// heartbeats plus one per leader-election participant.
	register("coordination.k8s.io", "v1", "leases")

	// discovery.k8s.io — EndpointSlices replace Endpoints; can be high-volume
	// in large clusters (multiple slices per service, updated on pod IP changes).
	register("discovery.k8s.io", "v1", "endpointslices")

	// rbac.authorization.k8s.io/v1
	register("rbac.authorization.k8s.io", "v1", "clusterroles")
	register("rbac.authorization.k8s.io", "v1", "clusterrolebindings")
	register("rbac.authorization.k8s.io", "v1", "roles")
	register("rbac.authorization.k8s.io", "v1", "rolebindings")

	// policy/v1
	register("policy", "v1", "poddisruptionbudgets")

	// admissionregistration.k8s.io/v1
	register("admissionregistration.k8s.io", "v1", "mutatingwebhookconfigurations")
	register("admissionregistration.k8s.io", "v1", "validatingwebhookconfigurations")
	register("admissionregistration.k8s.io", "v1", "validatingadmissionpolicies")
	register("admissionregistration.k8s.io", "v1", "validatingadmissionpolicybindings")

	// storage.k8s.io/v1
	register("storage.k8s.io", "v1", "storageclasses")
	register("storage.k8s.io", "v1", "volumeattachments")
	register("storage.k8s.io", "v1", "csidrivers")
	register("storage.k8s.io", "v1", "csinodes")
	register("storage.k8s.io", "v1", "csistoragecapacities")

	// certificates.k8s.io
	register("certificates.k8s.io", "v1", "certificatesigningrequests")
	register("certificates.k8s.io", "v1beta1", "clustertrustbundles")

	// events.k8s.io/v1 — distinct from core/v1 events; the newer Events API.
	register("events.k8s.io", "v1", "events")

	// flowcontrol.apiserver.k8s.io/v1 — API Priority and Fairness
	register("flowcontrol.apiserver.k8s.io", "v1", "flowschemas")
	register("flowcontrol.apiserver.k8s.io", "v1", "prioritylevelconfigurations")

	// node.k8s.io/v1
	register("node.k8s.io", "v1", "runtimeclasses")

	// scheduling.k8s.io/v1
	register("scheduling.k8s.io", "v1", "priorityclasses")

	// resource.k8s.io — Dynamic Resource Allocation (DRA); beta in 1.30+
	register("resource.k8s.io", "v1beta1", "deviceclasses")
	register("resource.k8s.io", "v1beta1", "resourceclaims")
	register("resource.k8s.io", "v1beta1", "resourceclaimtemplates")
	register("resource.k8s.io", "v1beta1", "resourceslices")
	register("resource.k8s.io", "v1beta2", "deviceclasses")
	register("resource.k8s.io", "v1beta2", "resourceclaims")
	register("resource.k8s.io", "v1beta2", "resourceclaimtemplates")
	register("resource.k8s.io", "v1beta2", "resourceslices")

	// internal.apiserver.k8s.io/v1alpha1 — StorageVersion (API server internal)
	register("internal.apiserver.k8s.io", "v1alpha1", "storageversions")

	// storagemigration.k8s.io/v1alpha1
	register("storagemigration.k8s.io", "v1alpha1", "storageversionmigrations")
}
