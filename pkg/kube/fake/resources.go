package fake

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func FindGvr(resources []*metav1.APIResourceList, apiVersion, kindOrName string) *schema.GroupVersionResource {
	for _, apiResourceGroup := range resources {
		if apiVersion != "" && apiResourceGroup.GroupVersion != apiVersion {
			continue
		}
		for _, apiResource := range apiResourceGroup.APIResources {
			if strings.ToLower(apiResource.Kind) == strings.ToLower(kindOrName) || strings.ToLower(apiResource.Name) == strings.ToLower(kindOrName) {
				// ignore parse error, because FakeClusterResources should be valid
				gv, _ := schema.ParseGroupVersion(apiResourceGroup.GroupVersion)
				return &schema.GroupVersionResource{
					Resource: apiResource.Name,
					Group:    gv.Group,
					Version:  gv.Version,
				}
			}
		}
	}
	return nil
}

func ClusterResources() []*metav1.APIResourceList {
	return []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "Binding",
					Name:       "bindings",
					Verbs:      metav1.Verbs{"create"},
					Group:      "",
					Version:    "v1",
					Namespaced: true,
				},
				{
					Kind:       "ComponentStatus",
					Name:       "componentstatuses",
					Verbs:      metav1.Verbs{"get", "list"},
					Group:      "",
					Version:    "v1",
					Namespaced: false,
				},
				{
					Kind:       "ConfigMap",
					Name:       "configmaps",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "",
					Version:    "v1",
					Namespaced: true,
				},
				{
					Kind:       "Endpoints",
					Name:       "endpoints",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "",
					Version:    "v1",
					Namespaced: true,
				},
				{
					Kind:       "Event",
					Name:       "events",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "",
					Version:    "v1",
					Namespaced: true,
				},
				{
					Kind:       "LimitRange",
					Name:       "limitranges",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "",
					Version:    "v1",
					Namespaced: true,
				},
				{
					Kind:       "Namespace",
					Name:       "namespaces",
					Verbs:      metav1.Verbs{"create", "delete", "get", "list", "patch", "update", "watch"},
					Group:      "",
					Version:    "v1",
					Namespaced: false,
				},
				{
					Kind:       "Node",
					Name:       "nodes",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "",
					Version:    "v1",
					Namespaced: false,
				},
				{
					Kind:       "PersistentVolumeClaim",
					Name:       "persistentvolumeclaims",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "",
					Version:    "v1",
					Namespaced: true,
				},
				{
					Kind:       "PersistentVolume",
					Name:       "persistentvolumes",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "",
					Version:    "v1",
					Namespaced: false,
				},
				{
					Kind:       "Pod",
					Name:       "pods",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "",
					Version:    "v1",
					Namespaced: true,
				},
				{
					Kind:       "PodTemplate",
					Name:       "podtemplates",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "",
					Version:    "v1",
					Namespaced: true,
				},
				{
					Kind:       "ReplicationController",
					Name:       "replicationcontrollers",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "",
					Version:    "v1",
					Namespaced: true,
				},
				{
					Kind:       "ResourceQuota",
					Name:       "resourcequotas",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "",
					Version:    "v1",
					Namespaced: true,
				},
				{
					Kind:       "Secret",
					Name:       "secrets",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "",
					Version:    "v1",
					Namespaced: true,
				},
				{
					Kind:       "ServiceAccount",
					Name:       "serviceaccounts",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "",
					Version:    "v1",
					Namespaced: true,
				},
				{
					Kind:       "Service",
					Name:       "services",
					Verbs:      metav1.Verbs{"create", "delete", "get", "list", "patch", "update", "watch"},
					Group:      "",
					Version:    "v1",
					Namespaced: true,
				},
			},
		},
		{
			GroupVersion: "apiregistration.k8s.io/v1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "APIService",
					Name:       "apiservices",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "apiregistration.k8s.io",
					Version:    "v1",
					Namespaced: false,
				},
			},
		},
		{
			GroupVersion: "apiregistration.k8s.io/v1beta1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "APIService",
					Name:       "apiservices",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "apiregistration.k8s.io",
					Version:    "v1beta1",
					Namespaced: false,
				},
			},
		},
		{
			GroupVersion: "extensions/v1beta1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "DaemonSet",
					Name:       "daemonsets",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "extensions",
					Version:    "v1beta1",
					Namespaced: true,
				},
				{
					Kind:       "Deployment",
					Name:       "deployments",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "extensions",
					Version:    "v1beta1",
					Namespaced: true,
				},
				{
					Kind:       "Ingress",
					Name:       "ingresses",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "extensions",
					Version:    "v1beta1",
					Namespaced: true,
				},
				{
					Kind:       "NetworkPolicy",
					Name:       "networkpolicies",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "extensions",
					Version:    "v1beta1",
					Namespaced: true,
				},
				{
					Kind:       "PodSecurityPolicy",
					Name:       "podsecuritypolicies",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "extensions",
					Version:    "v1beta1",
					Namespaced: false,
				},
				{
					Kind:       "ReplicaSet",
					Name:       "replicasets",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "extensions",
					Version:    "v1beta1",
					Namespaced: true,
				},
				{
					Kind:       "ReplicationControllerDummy",
					Name:       "replicationcontrollers",
					Verbs:      metav1.Verbs{},
					Group:      "extensions",
					Version:    "v1beta1",
					Namespaced: true,
				},
			},
		},
		{
			GroupVersion: "apiregistration.k8s.io/v1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "APIService",
					Name:       "apiservices",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "apiregistration.k8s.io",
					Version:    "v1",
					Namespaced: false,
				},
			},
		},
		{
			GroupVersion: "apiregistration.k8s.io/v1beta1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "APIService",
					Name:       "apiservices",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "apiregistration.k8s.io",
					Version:    "v1beta1",
					Namespaced: false,
				},
			},
		},
		{
			GroupVersion: "extensions/v1beta1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "DaemonSet",
					Name:       "daemonsets",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "extensions",
					Version:    "v1beta1",
					Namespaced: true,
				},
				{
					Kind:       "Deployment",
					Name:       "deployments",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "extensions",
					Version:    "v1beta1",
					Namespaced: true,
				},
				{
					Kind:       "Ingress",
					Name:       "ingresses",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "extensions",
					Version:    "v1beta1",
					Namespaced: true,
				},
				{
					Kind:       "NetworkPolicy",
					Name:       "networkpolicies",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "extensions",
					Version:    "v1beta1",
					Namespaced: true,
				},
				{
					Kind:       "PodSecurityPolicy",
					Name:       "podsecuritypolicies",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "extensions",
					Version:    "v1beta1",
					Namespaced: false,
				},
				{
					Kind:       "ReplicaSet",
					Name:       "replicasets",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "extensions",
					Version:    "v1beta1",
					Namespaced: true,
				},
				{
					Kind:       "ReplicationControllerDummy",
					Name:       "replicationcontrollers",
					Verbs:      metav1.Verbs{},
					Group:      "extensions",
					Version:    "v1beta1",
					Namespaced: true,
				},
			},
		},
		{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "ControllerRevision",
					Name:       "controllerrevisions",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "apps",
					Version:    "v1",
					Namespaced: true,
				},
				{
					Kind:       "DaemonSet",
					Name:       "daemonsets",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "apps",
					Version:    "v1",
					Namespaced: true,
				},
				{
					Kind:       "Deployment",
					Name:       "deployments",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "apps",
					Version:    "v1",
					Namespaced: true,
				},
				{
					Kind:       "ReplicaSet",
					Name:       "replicasets",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "apps",
					Version:    "v1",
					Namespaced: true,
				},
				{
					Kind:       "StatefulSet",
					Name:       "statefulsets",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "apps",
					Version:    "v1",
					Namespaced: true,
				},
			},
		},
		{
			GroupVersion: "apps/v1beta2",
			APIResources: []metav1.APIResource{
				{
					Kind:       "ControllerRevision",
					Name:       "controllerrevisions",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "apps",
					Version:    "v1beta2",
					Namespaced: true,
				},
				{
					Kind:       "DaemonSet",
					Name:       "daemonsets",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "apps",
					Version:    "v1beta2",
					Namespaced: true,
				},
				{
					Kind:       "Deployment",
					Name:       "deployments",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "apps",
					Version:    "v1beta2",
					Namespaced: true,
				},
				{
					Kind:       "ReplicaSet",
					Name:       "replicasets",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "apps",
					Version:    "v1beta2",
					Namespaced: true,
				},
				{
					Kind:       "StatefulSet",
					Name:       "statefulsets",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "apps",
					Version:    "v1beta2",
					Namespaced: true,
				},
			},
		},
		{
			GroupVersion: "apps/v1beta1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "ControllerRevision",
					Name:       "controllerrevisions",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "apps",
					Version:    "v1beta1",
					Namespaced: true,
				},
				{
					Kind:       "Deployment",
					Name:       "deployments",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "apps",
					Version:    "v1beta1",
					Namespaced: true,
				},
				{
					Kind:       "StatefulSet",
					Name:       "statefulsets",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "apps",
					Version:    "v1beta1",
					Namespaced: true,
				},
			},
		},
		{
			GroupVersion: "events.k8s.io/v1beta1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "Event",
					Name:       "events",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "events.k8s.io",
					Version:    "v1beta1",
					Namespaced: true,
				},
			},
		},
		{
			GroupVersion: "authentication.k8s.io/v1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "TokenReview",
					Name:       "tokenreviews",
					Verbs:      metav1.Verbs{"create"},
					Group:      "authentication.k8s.io",
					Version:    "v1",
					Namespaced: false,
				},
			},
		},
		{
			GroupVersion: "authentication.k8s.io/v1beta1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "TokenReview",
					Name:       "tokenreviews",
					Verbs:      metav1.Verbs{"create"},
					Group:      "authentication.k8s.io",
					Version:    "v1beta1",
					Namespaced: false,
				},
			},
		},
		{
			GroupVersion: "authorization.k8s.io/v1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "LocalSubjectAccessReview",
					Name:       "localsubjectaccessreviews",
					Verbs:      metav1.Verbs{"create"},
					Group:      "authorization.k8s.io",
					Version:    "v1",
					Namespaced: true,
				},
				{
					Kind:       "SelfSubjectAccessReview",
					Name:       "selfsubjectaccessreviews",
					Verbs:      metav1.Verbs{"create"},
					Group:      "authorization.k8s.io",
					Version:    "v1",
					Namespaced: false,
				},
				{
					Kind:       "SelfSubjectRulesReview",
					Name:       "selfsubjectrulesreviews",
					Verbs:      metav1.Verbs{"create"},
					Group:      "authorization.k8s.io",
					Version:    "v1",
					Namespaced: false,
				},
				{
					Kind:       "SubjectAccessReview",
					Name:       "subjectaccessreviews",
					Verbs:      metav1.Verbs{"create"},
					Group:      "authorization.k8s.io",
					Version:    "v1",
					Namespaced: false,
				},
			},
		},
		{
			GroupVersion: "authorization.k8s.io/v1beta1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "LocalSubjectAccessReview",
					Name:       "localsubjectaccessreviews",
					Verbs:      metav1.Verbs{"create"},
					Group:      "authorization.k8s.io",
					Version:    "v1beta1",
					Namespaced: true,
				},
				{
					Kind:       "SelfSubjectAccessReview",
					Name:       "selfsubjectaccessreviews",
					Verbs:      metav1.Verbs{"create"},
					Group:      "authorization.k8s.io",
					Version:    "v1beta1",
					Namespaced: false,
				},
				{
					Kind:       "SelfSubjectRulesReview",
					Name:       "selfsubjectrulesreviews",
					Verbs:      metav1.Verbs{"create"},
					Group:      "authorization.k8s.io",
					Version:    "v1beta1",
					Namespaced: false,
				},
				{
					Kind:       "SubjectAccessReview",
					Name:       "subjectaccessreviews",
					Verbs:      metav1.Verbs{"create"},
					Group:      "authorization.k8s.io",
					Version:    "v1beta1",
					Namespaced: false,
				},
			},
		},
		{
			GroupVersion: "autoscaling/v1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "HorizontalPodAutoscaler",
					Name:       "horizontalpodautoscalers",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "autoscaling",
					Version:    "v1",
					Namespaced: true,
				},
			},
		},
		{
			GroupVersion: "autoscaling/v2beta1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "HorizontalPodAutoscaler",
					Name:       "horizontalpodautoscalers",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "autoscaling",
					Version:    "v2beta1",
					Namespaced: true,
				},
			},
		},
		{
			GroupVersion: "autoscaling/v2beta2",
			APIResources: []metav1.APIResource{
				{
					Kind:       "HorizontalPodAutoscaler",
					Name:       "horizontalpodautoscalers",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "autoscaling",
					Version:    "v2beta2",
					Namespaced: true,
				},
			},
		},
		{
			GroupVersion: "batch/v1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "Job",
					Name:       "jobs",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "batch",
					Version:    "v1",
					Namespaced: true,
				},
			},
		},
		{
			GroupVersion: "batch/v1beta1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "CronJob",
					Name:       "cronjobs",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "batch",
					Version:    "v1beta1",
					Namespaced: true,
				},
			},
		},
		{
			GroupVersion: "certificates.k8s.io/v1beta1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "CertificateSigningRequest",
					Name:       "certificatesigningrequests",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "certificates.k8s.io",
					Version:    "v1beta1",
					Namespaced: false,
				},
			},
		},
		{
			GroupVersion: "networking.k8s.io/v1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "NetworkPolicy",
					Name:       "networkpolicies",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "networking.k8s.io",
					Version:    "v1",
					Namespaced: true,
				},
			},
		},
		{
			GroupVersion: "networking.k8s.io/v1beta1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "Ingress",
					Name:       "ingresses",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "networking.k8s.io",
					Version:    "v1beta1",
					Namespaced: true,
				},
			},
		},
		{
			GroupVersion: "policy/v1beta1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "PodDisruptionBudget",
					Name:       "poddisruptionbudgets",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "policy",
					Version:    "v1beta1",
					Namespaced: true,
				},
				{
					Kind:       "PodSecurityPolicy",
					Name:       "podsecuritypolicies",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "policy",
					Version:    "v1beta1",
					Namespaced: false,
				},
			},
		},
		{
			GroupVersion: "rbac.authorization.k8s.io/v1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "ClusterRoleBinding",
					Name:       "clusterrolebindings",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "rbac.authorization.k8s.io",
					Version:    "v1",
					Namespaced: false,
				},
				{
					Kind:       "ClusterRole",
					Name:       "clusterroles",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "rbac.authorization.k8s.io",
					Version:    "v1",
					Namespaced: false,
				},
				{
					Kind:       "RoleBinding",
					Name:       "rolebindings",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "rbac.authorization.k8s.io",
					Version:    "v1",
					Namespaced: true,
				},
				{
					Kind:       "Role",
					Name:       "roles",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "rbac.authorization.k8s.io",
					Version:    "v1",
					Namespaced: true,
				},
			},
		},
		{
			GroupVersion: "rbac.authorization.k8s.io/v1beta1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "ClusterRoleBinding",
					Name:       "clusterrolebindings",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "rbac.authorization.k8s.io",
					Version:    "v1beta1",
					Namespaced: false,
				},
				{
					Kind:       "ClusterRole",
					Name:       "clusterroles",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "rbac.authorization.k8s.io",
					Version:    "v1beta1",
					Namespaced: false,
				},
				{
					Kind:       "RoleBinding",
					Name:       "rolebindings",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "rbac.authorization.k8s.io",
					Version:    "v1beta1",
					Namespaced: true,
				},
				{
					Kind:       "Role",
					Name:       "roles",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "rbac.authorization.k8s.io",
					Version:    "v1beta1",
					Namespaced: true,
				},
			},
		},
		{
			GroupVersion: "storage.k8s.io/v1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "StorageClass",
					Name:       "storageclasses",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "storage.k8s.io",
					Version:    "v1",
					Namespaced: false,
				},
				{
					Kind:       "VolumeAttachment",
					Name:       "volumeattachments",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "storage.k8s.io",
					Version:    "v1",
					Namespaced: false,
				},
			},
		},
		{
			GroupVersion: "storage.k8s.io/v1beta1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "CSIDriver",
					Name:       "csidrivers",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "storage.k8s.io",
					Version:    "v1beta1",
					Namespaced: false,
				},
				{
					Kind:       "CSINode",
					Name:       "csinodes",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "storage.k8s.io",
					Version:    "v1beta1",
					Namespaced: false,
				},
				{
					Kind:       "StorageClass",
					Name:       "storageclasses",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "storage.k8s.io",
					Version:    "v1beta1",
					Namespaced: false,
				},
				{
					Kind:       "VolumeAttachment",
					Name:       "volumeattachments",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "storage.k8s.io",
					Version:    "v1beta1",
					Namespaced: false,
				},
			},
		},
		{
			GroupVersion: "admissionregistration.k8s.io/v1beta1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "MutatingWebhookConfiguration",
					Name:       "mutatingwebhookconfigurations",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "admissionregistration.k8s.io",
					Version:    "v1beta1",
					Namespaced: false,
				},
				{
					Kind:       "ValidatingWebhookConfiguration",
					Name:       "validatingwebhookconfigurations",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "admissionregistration.k8s.io",
					Version:    "v1beta1",
					Namespaced: false,
				},
			},
		},
		{
			GroupVersion: "apiextensions.k8s.io/v1beta1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "CustomResourceDefinition",
					Name:       "customresourcedefinitions",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "apiextensions.k8s.io",
					Version:    "v1beta1",
					Namespaced: false,
				},
			},
		},
		{
			GroupVersion: "scheduling.k8s.io/v1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "PriorityClass",
					Name:       "priorityclasses",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "scheduling.k8s.io",
					Version:    "v1",
					Namespaced: false,
				},
			},
		},
		{
			GroupVersion: "scheduling.k8s.io/v1beta1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "PriorityClass",
					Name:       "priorityclasses",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "scheduling.k8s.io",
					Version:    "v1beta1",
					Namespaced: false,
				},
			},
		},
		{
			GroupVersion: "coordination.k8s.io/v1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "Lease",
					Name:       "leases",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "coordination.k8s.io",
					Version:    "v1",
					Namespaced: true,
				},
			},
		},
		{
			GroupVersion: "coordination.k8s.io/v1beta1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "Lease",
					Name:       "leases",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "coordination.k8s.io",
					Version:    "v1beta1",
					Namespaced: true,
				},
			},
		},
		{
			GroupVersion: "node.k8s.io/v1beta1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "RuntimeClass",
					Name:       "runtimeclasses",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "node.k8s.io",
					Version:    "v1beta1",
					Namespaced: false,
				},
			},
		},
		{
			GroupVersion: "snapshot.storage.k8s.io/v1alpha1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "VolumeSnapshotClass",
					Name:       "volumesnapshotclasses",
					Verbs:      metav1.Verbs{"delete", "deletecollection", "get", "list", "patch", "create", "update", "watch"},
					Group:      "snapshot.storage.k8s.io",
					Version:    "v1alpha1",
					Namespaced: false,
				},
				{
					Kind:       "VolumeSnapshotContent",
					Name:       "volumesnapshotcontents",
					Verbs:      metav1.Verbs{"delete", "deletecollection", "get", "list", "patch", "create", "update", "watch"},
					Group:      "snapshot.storage.k8s.io",
					Version:    "v1alpha1",
					Namespaced: false,
				},
				{
					Kind:       "VolumeSnapshot",
					Name:       "volumesnapshots",
					Verbs:      metav1.Verbs{"delete", "deletecollection", "get", "list", "patch", "create", "update", "watch"},
					Group:      "snapshot.storage.k8s.io",
					Version:    "v1alpha1",
					Namespaced: true,
				},
			},
		},
		{
			GroupVersion: "autoscaling.k8s.io/v1beta2",
			APIResources: []metav1.APIResource{
				{
					Kind:       "VerticalPodAutoscaler",
					Name:       "verticalpodautoscalers",
					Verbs:      metav1.Verbs{"delete", "deletecollection", "get", "list", "patch", "create", "update", "watch"},
					Group:      "autoscaling.k8s.io",
					Version:    "v1beta2",
					Namespaced: true,
				},
				{
					Kind:       "VerticalPodAutoscalerCheckpoint",
					Name:       "verticalpodautoscalercheckpoints",
					Verbs:      metav1.Verbs{"delete", "deletecollection", "get", "list", "patch", "create", "update", "watch"},
					Group:      "autoscaling.k8s.io",
					Version:    "v1beta2",
					Namespaced: true,
				},
			},
		},
		{
			GroupVersion: "autoscaling.k8s.io/v1beta1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "VerticalPodAutoscaler",
					Name:       "verticalpodautoscalers",
					Verbs:      metav1.Verbs{"delete", "deletecollection", "get", "list", "patch", "create", "update", "watch"},
					Group:      "autoscaling.k8s.io",
					Version:    "v1beta1",
					Namespaced: true,
				},
				{
					Kind:       "VerticalPodAutoscalerCheckpoint",
					Name:       "verticalpodautoscalercheckpoints",
					Verbs:      metav1.Verbs{"delete", "deletecollection", "get", "list", "patch", "create", "update", "watch"},
					Group:      "autoscaling.k8s.io",
					Version:    "v1beta1",
					Namespaced: true,
				},
			},
		},
		{
			GroupVersion: "metrics.k8s.io/v1beta1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "NodeMetrics",
					Name:       "nodes",
					Verbs:      metav1.Verbs{"get", "list"},
					Group:      "metrics.k8s.io",
					Version:    "v1beta1",
					Namespaced: false,
				},
				{
					Kind:       "PodMetrics",
					Name:       "pods",
					Verbs:      metav1.Verbs{"get", "list"},
					Group:      "metrics.k8s.io",
					Version:    "v1beta1",
					Namespaced: true,
				},
			},
		},
	}
}
