package service

import (
	log "github.com/sirupsen/logrus"

	_ "github.com/flant/shell-operator/pkg/utils/klogtologrus"
	"github.com/flant/shell-operator/pkg/utils/klogtologrus/test/klog-powered-lib"
)

func DoWithCallToKlogPoweredLib() {
	log.Trace("service action")

	klog_powered_lib.ActionWithKlogWarn()
	klog_powered_lib.ActionWithKlogInfo()
	klog_powered_lib.ActionWithKlogError()
}
