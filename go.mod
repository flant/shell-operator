module github.com/flant/shell-operator

go 1.16

require (
	github.com/flant/kube-client v0.0.6
	github.com/flant/libjq-go v1.6.2-0.20200616114952-907039e8a02a // branch: master
	github.com/go-chi/chi v4.0.3+incompatible
	github.com/go-openapi/spec v0.19.8
	github.com/go-openapi/strfmt v0.19.5
	github.com/go-openapi/swag v0.19.9
	github.com/go-openapi/validate v0.19.12
	github.com/hashicorp/go-multierror v1.1.1
	github.com/kennygrant/sanitize v1.2.4
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.19.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.0
	github.com/sirupsen/logrus v1.8.1
	github.com/stretchr/testify v1.7.0
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/robfig/cron.v2 v2.0.0-20150107220207-be2e0b0deed5
	gopkg.in/satori/go.uuid.v1 v1.2.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	k8s.io/api v0.19.11
	k8s.io/apiextensions-apiserver v0.19.11
	k8s.io/apimachinery v0.19.11
	k8s.io/client-go v0.19.11
	sigs.k8s.io/yaml v1.2.0
)

// Remove 'in body' from errors, fix for Go 1.16 (https://github.com/go-openapi/validate/pull/138).
replace github.com/go-openapi/validate => github.com/flant/go-openapi-validate v0.19.12-flant.0

require golang.org/x/crypto v0.0.0-20220315160706-3147a52a75dd // indirect
