module github.com/flant/shell-operator

go 1.12

require (
	github.com/davecgh/go-spew v1.1.1
	github.com/flant/libjq-go v1.6.2-0.20200616114952-907039e8a02a // branch: master
	github.com/go-chi/chi v4.0.3+incompatible
	github.com/go-openapi/spec v0.19.3
	github.com/go-openapi/strfmt v0.19.3
	github.com/go-openapi/swag v0.19.5
	github.com/go-openapi/validate v0.19.7
	github.com/hashicorp/go-multierror v1.0.0
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/kennygrant/sanitize v1.2.4
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.9.0
	github.com/prometheus/client_golang v1.7.1
	github.com/sirupsen/logrus v1.6.0
	github.com/stretchr/testify v1.4.0
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/robfig/cron.v2 v2.0.0-20150107220207-be2e0b0deed5
	gopkg.in/satori/go.uuid.v1 v1.2.0
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776
	k8s.io/api v0.19.2
	k8s.io/apiextensions-apiserver v0.19.2
	k8s.io/apimachinery v0.20.5 // version is bumped to please Go 1.16 (package ... imported from implicitly required module)
	k8s.io/client-go v0.19.2
	k8s.io/klog v1.0.0
	sigs.k8s.io/kind v0.10.0
	sigs.k8s.io/yaml v1.2.0
)

// replace github.com/go-openapi/validate => ../go-openapi-validate
replace github.com/go-openapi/validate => github.com/flant/go-openapi-validate v0.19.4-0.20200313141509-0c0fba4d39e1 // branch: fix_in_body_0_19_7

replace github.com/onsi/ginkgo v1.11.0 => github.com/flant/ginkgo v1.11.1-0.20200206071017-2216da3b016c // fix_coverage_combining

//replace github.com/flant/libjq-go => ../libjq-go

// version from k8s.io/client-go
//replace k8s.io/klog => k8s.io/klog v0.0.0-20190306015804-8e90cee79f82

// kind 0.10.0 requires 0.19.2
replace k8s.io/apimachinery => k8s.io/apimachinery v0.19.2
