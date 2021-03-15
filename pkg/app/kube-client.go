package app

import (
	"time"

	"gopkg.in/alecthomas/kingpin.v2"
)

var KubeContext = ""
var KubeConfig = ""
var KubeServer = ""
var KubeClientQpsDefault = "5" // DefaultQPS from k8s.io/client-go/rest/config.go
var KubeClientQps float32
var KubeClientBurstDefault = "10" // DefaultBurst from k8s.io/client-go/rest/config.go
var KubeClientBurst int
var KubeClientTimeoutDefault = "10"
var KubeClientTimeout time.Duration

func DefineKubeClientFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("kube-context", "The name of the kubeconfig context to use. Can be set with $KUBE_CONTEXT.").
		Envar("KUBE_CONTEXT").
		Default(KubeContext).
		StringVar(&KubeContext)

	cmd.Flag("kube-config", "Path to the kubeconfig file. Can be set with $KUBE_CONFIG.").
		Envar("KUBE_CONFIG").
		Default(KubeConfig).
		StringVar(&KubeConfig)

	// Rate limit settings for kube client
	cmd.Flag("kube-client-qps", "QPS for a rate limiter of a kubernetes client. Can be set with $KUBE_CLIENT_QPS.").
		Envar("KUBE_CLIENT_QPS").
		Default(KubeClientQpsDefault).
		Float32Var(&KubeClientQps)
	cmd.Flag("kube-client-burst", "Burst for a rate limiter of a kubernetes client. Can be set with $KUBE_CLIENT_BURST.").
		Envar("KUBE_CLIENT_BURST").
		Default(KubeClientBurstDefault).
		IntVar(&KubeClientBurst)

	cmd.Flag("kube-client-timeout", "Timeout for each request to the Kubernetes API server. Can be set with $KUBE_CLIENT_TIMEOUT").
		Envar("KUBE_CLIENT_TIMEOUT").
		Default(KubeClientTimeoutDefault).
		DurationVar(&KubeClientTimeout)

	cmd.Flag("kube-server", "The address and port of the Kubernetes API server. Can be set with $KUBE_SERVER.").
		Envar("KUBE_SERVER").
		Default(KubeServer).
		StringVar(&KubeServer)
}
