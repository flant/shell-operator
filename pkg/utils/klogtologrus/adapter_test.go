package klogtologrus

import (
	"encoding/json"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	log "github.com/sirupsen/logrus"
	"k8s.io/klog"
)

func Test_basic(t *testing.T) {
	g := NewWithT(t)

	buf := gbytes.NewBuffer()

	log.SetOutput(buf)
	log.SetFormatter(&log.JSONFormatter{DisableTimestamp: true})

	klog.Warning("test warning")

	var record map[string]string
	err := json.Unmarshal(buf.Contents(), &record)
	g.Expect(err).ShouldNot(HaveOccurred(), string(buf.Contents()))

	g.Expect(record).Should(HaveKey("level"))
	g.Expect(record["level"]).Should(Equal("warning"))
	g.Expect(record).Should(HaveKey("msg"))
	g.Expect(record["msg"]).Should(ContainSubstring("test warning"))
}
