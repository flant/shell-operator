package klog_powered_lib

import (
	"k8s.io/klog"
)

func ActionWithKlogWarn() {
	klog.Warning("Warning from klog powered lib")
}

func ActionWithKlogInfo() {
	klog.V(0).Info("Info from klog powered lib")
}

func ActionWithKlogError() {
	klog.ErrorDepth(2, "Error from klog powered lib")
}
