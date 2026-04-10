//go:build test
// +build test

package utils

import (
	"bufio"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/onsi/gomega/types"
)

type PromMetricValue struct {
	Name   string
	Labels map[string]string
	Value  float64
}

func (pmv PromMetricValue) String() string {
	labels := make([]string, 0, len(pmv.Labels))
	for k, v := range pmv.Labels {
		labels = append(labels, fmt.Sprintf("%s=%s", k, v))
	}
	l := ""
	if len(labels) > 0 {
		l = fmt.Sprintf("{%s}", strings.Join(labels, ","))
	}

	return fmt.Sprintf("%s%s %v", pmv.Name, l, pmv.Value)
}

func (pmv PromMetricValue) IsEmpty() bool {
	return pmv.Name == "" && len(pmv.Labels) == 0 && pmv.Value == 0.0
}

var EmptyPromMetricsValue = PromMetricValue{}

var metricMatchRe = regexp.MustCompile(`^([a-zA-Z_-]+)({(.*)})?\ ([0-9.eE+-])$`)

func ParsePromMetricValue(line string) PromMetricValue {
	parts := metricMatchRe.FindStringSubmatch(line)
	if parts == nil {
		return EmptyPromMetricsValue
	}

	// Last part is a float value
	value, err := strconv.ParseFloat(parts[len(parts)-1], 64)
	if err != nil {
		return EmptyPromMetricsValue
	}

	labels := map[string]string{}

	// labels should be in parts[3]
	if len(parts) > 3 {
		labels = ParsePromLabels(parts[3])
	}

	return PromMetricValue{
		Name:   parts[1],
		Labels: labels,
		Value:  value,
	}
}

func ParsePromLabels(labels string) map[string]string {
	res := map[string]string{}

	kvParts := strings.Split(labels, ",")

	for _, kvPart := range kvParts {
		kv := strings.SplitN(kvPart, "=", 2)
		if len(kv) == 2 {
			res[kv[0]] = kv[1]
		}
	}

	return res
}

type PromScraper struct {
	Address string
	Metrics []PromMetricValue
}

func NewPromScraper(address string) *PromScraper {
	return &PromScraper{
		Address: address,
	}
}

func (p *PromScraper) WithAddress(address string) {
	p.Address = address
}

func (p *PromScraper) Scrape() error {
	r, err := http.Get(p.Address)
	if err != nil {
		return err
	}

	defer func() {
		r.Body.Close()
	}()

	metrics := make([]PromMetricValue, 0)

	scanner := bufio.NewScanner(r.Body)
	for scanner.Scan() {
		pmv := ParsePromMetricValue(scanner.Text())
		if !pmv.IsEmpty() {
			metrics = append(metrics, pmv)
		}
	}

	p.Metrics = metrics

	fmt.Printf("Got metrics: \n")
	for _, pmv := range p.Metrics {
		fmt.Printf("%s\n", pmv)
	}

	return nil
}

func (p *PromScraper) HasMetric(_ string) {
}

func (p *PromScraper) MetricEquals(_ string) {
}

// FindExact searches metric by name and labels
func (p *PromScraper) FindExact(pmv PromMetricValue) PromMetricValue {
	for _, m := range p.Metrics {
		if m.Name == pmv.Name {
			allLabelsEqual := true
			for k, v := range m.Labels {
				pmvl, ok := pmv.Labels[k]
				if !ok || pmvl != v {
					allLabelsEqual = false
				}
			}

			if allLabelsEqual {
				return m
			}
		}
	}

	return PromMetricValue{}
}

func PromMetric(name string, labels ...string) PromMetricValue {
	labelsMap := map[string]string{}

	for i := 0; i < len(labels); i += 2 {
		k := labels[i]
		v := ""
		if i+1 < len(labels) {
			v = labels[i+1]
		}
		labelsMap[k] = v
	}
	return PromMetricM(name, labelsMap)
}

func PromMetricM(name string, labels map[string]string) PromMetricValue {
	return PromMetricValue{
		Name:   name,
		Labels: labels,
	}
}

// gomega matchers

func HaveMetric(pmv PromMetricValue) types.GomegaMatcher {
	return &HaveMetricMatcher{
		pmv: pmv,
	}
}

type HaveMetricMatcher struct {
	pmv         PromMetricValue
	promScraper *PromScraper
}

func (matcher *HaveMetricMatcher) Match(actual interface{}) (bool, error) {
	promScraper, ok := actual.(*PromScraper)
	if !ok {
		return false, fmt.Errorf("HaveMetric must be passed a *PromScraper. Got %T\n", actual)
	}

	matcher.promScraper = promScraper

	return !promScraper.FindExact(matcher.pmv).IsEmpty(), nil
}

func (matcher *HaveMetricMatcher) FailureMessage(_ interface{}) string {
	return "Expected ShellOperator has metric"
}

func (matcher *HaveMetricMatcher) NegatedFailureMessage(_ interface{}) string {
	return "Expected ShellOperator has no metric"
}

func HaveMetricValue() types.GomegaMatcher {
	return nil
}
