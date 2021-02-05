package conversion

import (
	"github.com/flant/shell-operator/pkg/utils/string_helper"
)

// WebhookConfig
type WebhookConfig struct {
	Rules    []Rule
	CrdName  string // This name is used as a suffix to create different URLs for clientConfig in CRD.
	Metadata struct {
		Name         string
		DebugName    string
		LogLabels    map[string]string
		MetricLabels map[string]string
	}
}

type Rule struct {
	FromVersion string `json:"fromVersion"`
	ToVersion   string `json:"toVersion"`
}

func (r Rule) String() string {
	return r.FromVersion + "->" + r.ToVersion
}

func (r Rule) ShortFromVersion() string {
	return string_helper.TrimGroup(r.FromVersion)
}

func (r Rule) ShortToVersion() string {
	return string_helper.TrimGroup(r.ToVersion)
}
