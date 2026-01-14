package app

import (
	"time"

	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	KubeContext            = ""
	KubeConfig             = ""
	KubeServer             = ""
	KubeClientFieldManager = ""
)

var (
	KubeClientQps   float32
	KubeClientBurst int
)

const (
	KubeClientQpsDefault   = "5"  // DefaultQPS from k8s.io/client-go/rest/config.go
	KubeClientBurstDefault = "10" // DefaultBurst from k8s.io/client-go/rest/config.go
)

var (
	ObjectPatcherKubeClientQps     float32
	ObjectPatcherKubeClientBurst   int
	ObjectPatcherKubeClientTimeout time.Duration
)

const (
	ObjectPatcherKubeClientQpsDefault     = "5"  // DefaultQPS from k8s.io/client-go/rest/config.go
	ObjectPatcherKubeClientBurstDefault   = "10" // DefaultBurst from k8s.io/client-go/rest/config.go
	ObjectPatcherKubeClientTimeoutDefault = "10s"
)

func DefineKubeClientFlags(cmd *kingpin.CmdClause) {
	// Settings for Kubernetes connection.
	cmd.Flag("kube-context", "The name of the kubeconfig context to use. Can be set with $KUBE_CONTEXT.").
		Envar("KUBE_CONTEXT").
		Default(KubeContext).
		StringVar(&KubeContext)
	cmd.Flag("kube-config", "Path to the kubeconfig file. Can be set with $KUBE_CONFIG.").
		Envar("KUBE_CONFIG").
		Default(KubeConfig).
		StringVar(&KubeConfig)
	cmd.Flag("kube-server", "The address and port of the Kubernetes API server. Can be set with $KUBE_SERVER.").
		Envar("KUBE_SERVER").
		Default(KubeServer).
		StringVar(&KubeServer)

	// Rate limit settings for 'main' kube client
	cmd.Flag("kube-client-qps", "QPS for a rate limiter of a Kubernetes client for hook events. Can be set with $KUBE_CLIENT_QPS.").
		Envar("KUBE_CLIENT_QPS").
		Default(KubeClientQpsDefault).
		Float32Var(&KubeClientQps)
	cmd.Flag("kube-client-burst", "Burst for a rate limiter of a Kubernetes client for hook events. Can be set with $KUBE_CLIENT_BURST.").
		Envar("KUBE_CLIENT_BURST").
		Default(KubeClientBurstDefault).
		IntVar(&KubeClientBurst)

	// Settings for 'object_patcher' kube client
	cmd.Flag("object-patcher-kube-client-qps", "QPS for a rate limiter of a Kubernetes client for Object patcher. Can be set with $OBJECT_PATCHER_KUBE_CLIENT_QPS.").
		Envar("OBJECT_PATCHER_KUBE_CLIENT_QPS").
		Default(ObjectPatcherKubeClientQpsDefault).
		Float32Var(&ObjectPatcherKubeClientQps)
	cmd.Flag("object-patcher-kube-client-burst", "Burst for a rate limiter of a Kubernetes client for Object patcher. Can be set with $OBJECT_PATCHER_KUBE_CLIENT_BURST.").
		Envar("OBJECT_PATCHER_KUBE_CLIENT_BURST").
		Default(ObjectPatcherKubeClientBurstDefault).
		IntVar(&ObjectPatcherKubeClientBurst)
	cmd.Flag("object-patcher-kube-client-timeout", "Timeout for object patcher requests to the Kubernetes API server. Can be set with $OBJECT_PATCHER_KUBE_CLIENT_TIMEOUT").
		Envar("OBJECT_PATCHER_KUBE_CLIENT_TIMEOUT").
		Default(ObjectPatcherKubeClientTimeoutDefault).
		DurationVar(&ObjectPatcherKubeClientTimeout)
}
