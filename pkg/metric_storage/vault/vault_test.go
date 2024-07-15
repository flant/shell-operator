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
	v.Registerer = prometheus.DefaultRegisterer

	v.CounterAdd("group1", "metric_total", 1.0, map[string]string{"lbl": "val"})

	g.Expect(buf.String()).ShouldNot(ContainSubstring("error"), "error occurred in log: %s", buf.String())

	expect := `
# HELP metric_total metric_total
# TYPE metric_total counter
metric_total{lbl="val"} 1
`
	err := promtest.GatherAndCompare(prometheus.DefaultGatherer, strings.NewReader(expect), "metric_total")
	g.Expect(err).ShouldNot(HaveOccurred())

	v.ExpireGroupMetrics("group1")

	g.Expect(buf.String()).ShouldNot(ContainSubstring("error"), "error occurred in log: %s", buf.String())

	// Expect no metric with lbl="val"
	expect = ``
	err = promtest.GatherAndCompare(prometheus.DefaultGatherer, strings.NewReader(expect), "metric_total")
	g.Expect(err).ShouldNot(HaveOccurred())

	v.CounterAdd("group1", "metric_total", 1.0, map[string]string{"lbl": "val2"})

	g.Expect(buf.String()).ShouldNot(ContainSubstring("error"), "error occurred in log: %s", buf.String())

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

	g.Expect(buf.String()).ShouldNot(ContainSubstring("error"), "error occurred in log: %s", buf.String())

	// Expect metric_total with 2 label values
	expect = `
# HELP metric_total metric_total
# TYPE metric_total counter
metric_total{lbl="val2"} 2
metric_total{lbl="val222"} 1
`
	err = promtest.GatherAndCompare(prometheus.DefaultGatherer, strings.NewReader(expect), "metric_total")
	g.Expect(err).ShouldNot(HaveOccurred())

	v.ExpireGroupMetrics("group1")
	v.CounterAdd("group1", "metric_total", 1.0, map[string]string{"lbl": "val"})
	v.CounterAdd("group2", "metric2_total", 1.0, map[string]string{"lbl": "val222"})

	g.Expect(buf.String()).ShouldNot(ContainSubstring("error"), "error occurred in log: %s", buf.String())
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

	v.ExpireGroupMetrics("group1")
	v.CounterAdd("group2", "metric2_total", 1.0, map[string]string{"lbl": "val222"})
	g.Expect(buf.String()).ShouldNot(ContainSubstring("error"), "error occurred in log: %s", buf.String())
	// Expect metric_total is updated and metric2_total is updated and metric_total left as is
	expect = `
# HELP metric2_total metric2_total
# TYPE metric2_total counter
metric2_total{lbl="val222"} 2
`
	err = promtest.GatherAndCompare(prometheus.DefaultGatherer, strings.NewReader(expect), "metric_total", "metric2_total")
	g.Expect(err).ShouldNot(HaveOccurred())

	v.ExpireGroupMetrics("group1")
	v.ExpireGroupMetrics("group2")

	// Expect all metric instances sharing the same name to share equal labelsets respectively

	v.GaugeSet("group1", "metric_total1", 1.0, map[string]string{"a": "A"})
	v.GaugeSet("group1", "metric_total1", 2.0, map[string]string{"c": "C"})
	v.GaugeSet("group1", "metric_total1", 3.0, map[string]string{"a": "A", "b": "B"})
	v.GaugeSet("group1", "metric_total1", 5.0, map[string]string{"a": "A"})
	v.GaugeSet("group1", "metric_total2", 1.0, map[string]string{"a": "A1"})
	v.GaugeSet("group1", "metric_total2", 2.0, map[string]string{"c": "C2"})
	v.GaugeSet("group1", "metric_total2", 3.0, map[string]string{"a": "A3", "b": "B3"})

	v.CounterAdd("group2", "metric_total3", 1.0, map[string]string{"lbl": "val222"})
	v.CounterAdd("group2", "metric_total3", 1.0, map[string]string{"ord": "ord222"})
	v.CounterAdd("group2", "metric_total3", 4.0, map[string]string{"lbl": "val222"})
	v.CounterAdd("group2", "metric_total3", 9.0, map[string]string{"ord": "ord222"})
	v.CounterAdd("group2", "metric_total3", 99.0, map[string]string{"lbl": "val222", "ord": "ord222"})
	v.CounterAdd("group2", "metric_total3", 9.0, map[string]string{"lbl": "val222", "ord": "ord222"})

	v.CounterAdd("group3", "metric_total4", 9.0, map[string]string{"d": "d1"})
	v.CounterAdd("group3", "metric_total4", 99.0, map[string]string{"a": "a1", "b": "b1", "c": "c1", "d": "d1"})
	v.CounterAdd("group3", "metric_total4", 19.0, map[string]string{"c": "c2"})
	v.CounterAdd("group3", "metric_total4", 29.0, map[string]string{})
	v.CounterAdd("group3", "metric_total4", 39.0, map[string]string{"j": "j1"})
	v.CounterAdd("group3", "metric_total4", 1.0, map[string]string{"j": "j1"})
	v.CounterAdd("group3", "metric_total4", 1.0, map[string]string{"a": "", "b": "", "c": "", "d": "", "j": "j1"})

	g.Expect(buf.String()).ShouldNot(ContainSubstring("error"), "error occurred in log: %s", buf.String())

	expect = `
# HELP metric_total1 metric_total1
# TYPE metric_total1 gauge
metric_total1{a="A", b="", c=""} 5
metric_total1{a="", b="", c="C"} 2
metric_total1{a="A", b="B", c=""} 3
# HELP metric_total2 metric_total2
# TYPE metric_total2 gauge
metric_total2{a="A1", b="", c=""} 1
metric_total2{a="", b="", c="C2"} 2
metric_total2{a="A3", b="B3", c=""} 3
# HELP metric_total3 metric_total3
# TYPE metric_total3 counter
metric_total3{lbl="val222", ord=""} 5
metric_total3{lbl="", ord="ord222"} 10
metric_total3{lbl="val222", ord="ord222"} 108
# HELP metric_total4 metric_total4
# TYPE metric_total4 counter
metric_total4{a="", b="", c="", d="d1", j=""} 9
metric_total4{a="a1", b="b1", c="c1", d="d1", j=""} 99
metric_total4{a="", b="", c="c2", d="", j=""} 19
metric_total4{a="", b="", c="", d="", j=""} 29
metric_total4{a="", b="", c="", d="", j="j1"} 41
`

	err = promtest.GatherAndCompare(prometheus.DefaultGatherer, strings.NewReader(expect), "metric_total1", "metric_total2", "metric_total3", "metric_total4")
	g.Expect(err).ShouldNot(HaveOccurred())
}
