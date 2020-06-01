package vault

import (
	"bytes"
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/prometheus/client_golang/prometheus"
	promtest "github.com/prometheus/client_golang/prometheus/testutil"
	log "github.com/sirupsen/logrus"
)

func Test_CounterAdd(t *testing.T) {
	g := NewWithT(t)

	buf := &bytes.Buffer{}
	log.SetOutput(buf)

	v := NewGroupedVault()

	v.CounterAdd("group1", "metric_total", 1.0, map[string]string{"lbl": "val"})

	g.Expect(buf.String()).ShouldNot(ContainSubstring("error"), "error occured in log: %s", buf.String())

	expect := `
# HELP metric_total metric_total
# TYPE metric_total counter
metric_total{lbl="val"} 1
`
	err := promtest.GatherAndCompare(prometheus.DefaultGatherer, strings.NewReader(expect), "metric_total")
	g.Expect(err).ShouldNot(HaveOccurred())

	v.ClearMissingMetrics("group1", map[string][]map[string]string{
		"metric_total": {
			{"lbl": "qwe"},
		},
	})

	g.Expect(buf.String()).ShouldNot(ContainSubstring("error"), "error occured in log: %s", buf.String())

	// Expect no metric with lbl="val"
	expect = ``
	err = promtest.GatherAndCompare(prometheus.DefaultGatherer, strings.NewReader(expect), "metric_total")
	g.Expect(err).ShouldNot(HaveOccurred())

	v.CounterAdd("group1", "metric_total", 1.0, map[string]string{"lbl": "val2"})

	g.Expect(buf.String()).ShouldNot(ContainSubstring("error"), "error occured in log: %s", buf.String())

	// Expect metric_total with new label value
	expect = `
# HELP metric_total metric_total
# TYPE metric_total counter
metric_total{lbl="val2"} 1
`
	err = promtest.GatherAndCompare(prometheus.DefaultGatherer, strings.NewReader(expect), "metric_total")
	g.Expect(err).ShouldNot(HaveOccurred())

	v.CounterAdd("group1", "metric_total", 1.0, map[string]string{"lbl": "val2"})
	v.CounterAdd("group1", "metric_total", 1.0, map[string]string{"lbl": "val222"})

	g.Expect(buf.String()).ShouldNot(ContainSubstring("error"), "error occured in log: %s", buf.String())

	// Expect metric_total with 2 label values
	expect = `
# HELP metric_total metric_total
# TYPE metric_total counter
metric_total{lbl="val2"} 2
metric_total{lbl="val222"} 1
`
	err = promtest.GatherAndCompare(prometheus.DefaultGatherer, strings.NewReader(expect), "metric_total")
	g.Expect(err).ShouldNot(HaveOccurred())

	v.ClearMissingMetrics("group1", map[string][]map[string]string{
		"metric_total": {
			{"lbl": "val"},
		},
		"metric2_total": {
			{"lbl222": "val222"},
		},
	})
	v.CounterAdd("group1", "metric_total", 1.0, map[string]string{"lbl": "val"})
	v.CounterAdd("group2", "metric2_total", 1.0, map[string]string{"lbl": "val222"})

	g.Expect(buf.String()).ShouldNot(ContainSubstring("error"), "error occured in log: %s", buf.String())
	// Expect metric_total is updated and metric2_total
	expect = `
# HELP metric_total metric_total
# TYPE metric_total counter
metric_total{lbl="val"} 1
# HELP metric2_total metric2_total
# TYPE metric2_total counter
metric2_total{lbl="val222"} 1
`
	err = promtest.GatherAndCompare(prometheus.DefaultGatherer, strings.NewReader(expect), "metric_total", "metric2_total")
	g.Expect(err).ShouldNot(HaveOccurred())

	v.ClearMissingMetrics("group1", map[string][]map[string]string{
		"metric2_total": {
			{"lbl222": "val222"},
		},
	})
	v.CounterAdd("group2", "metric2_total", 1.0, map[string]string{"lbl": "val222"})
	g.Expect(buf.String()).ShouldNot(ContainSubstring("error"), "error occured in log: %s", buf.String())
	// Expect metric_total is updated and metric2_total is updated and metric_total left as is
	expect = `
# HELP metric_total metric_total
# TYPE metric_total counter
metric_total{lbl="val"} 1
# HELP metric2_total metric2_total
# TYPE metric2_total counter
metric2_total{lbl="val222"} 2
`
	err = promtest.GatherAndCompare(prometheus.DefaultGatherer, strings.NewReader(expect), "metric_total", "metric2_total")
	g.Expect(err).ShouldNot(HaveOccurred())

}
