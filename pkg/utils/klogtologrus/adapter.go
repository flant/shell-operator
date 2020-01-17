package klogtologrus

import (
	"flag"

	log "github.com/sirupsen/logrus"
	"k8s.io/klog"
)

// Override output writer for klog to log messages with logrus.
//
// Usage:
//
//   import (
//     _ "github.com/flant/shell-operator/pkg/utils/klogtologrus"
//   )
func init() {
	// turn off logging to stderr and set Info writer to catch all messages.
	klogFlagSet := flag.NewFlagSet("klog", flag.ContinueOnError)
	klog.InitFlags(klogFlagSet)
	_ = klogFlagSet.Parse([]string{"-logtostderr=false"})
	klog.SetOutputBySeverity("INFO", &klogToLogrusWriter{})
}

type klogToLogrusWriter struct{}

func (w *klogToLogrusWriter) Write(msg []byte) (n int, err error) {
	switch msg[0] {
	case 'W':
		log.Warn(string(msg))
	case 'E':
		log.Error(string(msg))
	case 'F':
		log.Fatal(string(msg))
	default:
		log.Info(string(msg))
	}
	return 0, nil
}
