package app

// AppName and AppDescription are read-only product identifiers, safe to use
// from anywhere. AppStartMessage was previously a mutable global rewritten at
// startup; library consumers now build their own banner from AppName/Version.
var (
	AppName        = "shell-operator"
	AppDescription = "Run your custom cluster-wide scripts in reaction to Kubernetes events or on schedule."
)

var Version = "v1.2.0-dev"
