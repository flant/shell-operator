package service

import (
	log "github.com/sirupsen/logrus"

	klog_powered_lib "github.com/flant/shell-operator/pkg/utils/klogtologrus/test/klog-powered-lib"
)

func DoWithCallToKlogPoweredLib() {
	log.Trace("service action")

	klog_powered_lib.ActionWithKlogWarn()
	klog_powered_lib.ActionWithKlogInfo()
	klog_powered_lib.ActionWithKlogError()
}
