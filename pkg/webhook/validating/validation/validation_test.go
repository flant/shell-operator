package validation

import (
	"testing"

	. "github.com/onsi/gomega"
	v1 "k8s.io/api/admissionregistration/v1"
	"sigs.k8s.io/yaml"
)

func decodeValidatingFromYaml(inYaml string) *v1.ValidatingWebhookConfiguration {
	res := new(v1.ValidatingWebhookConfiguration)

	err := yaml.Unmarshal([]byte(inYaml), res)
	if err != nil {
		panic(err)
	}

	return res
}

func Test_Validate(t *testing.T) {
	g := NewWithT(t)

	validConfYaml := `
#apiVersion: admissionregistration.k8s.io/v1
#kind: ValidatingWebhookConfiguration
#metadata:
#  name: "pod-policy.example.com"
webhooks:
- name: "pod-policy.example.com"
  objectSelector:
    matchLabels:
      foo: bar
  rules:
  - apiGroups:   [""]
    apiVersions: ["v1"]
    operations:  ["CREATE"]
    resources:   ["pods"]
    scope:       "Namespaced"
  sideEffects: None
  timeoutSeconds: 5
`
	validConf := decodeValidatingFromYaml(validConfYaml)

	err := ValidateValidatingWebhooks(validConf)

	g.Expect(err).ShouldNot(HaveOccurred())
}
