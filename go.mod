module github.com/flant/shell-operator

go 1.12

require (
	github.com/flant/libjq-go v1.0.1-0.20200205115921-27e93c18c17f // downgrade jq to 1.6
	github.com/go-chi/chi v4.0.3+incompatible
	github.com/go-openapi/errors v0.19.2
	github.com/go-openapi/spec v0.19.3
	github.com/go-openapi/strfmt v0.19.2
	github.com/go-openapi/swag v0.19.5
	github.com/go-openapi/validate v0.19.3
	github.com/hashicorp/go-multierror v1.0.0
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/kennygrant/sanitize v1.2.4
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.9.0
	github.com/prometheus/client_golang v1.0.0
	github.com/sirupsen/logrus v1.4.2
	github.com/stretchr/testify v1.4.0
	golang.org/x/sys v0.0.0-20200113162924-86b910548bc1
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/robfig/cron.v2 v2.0.0-20150107220207-be2e0b0deed5
	gopkg.in/satori/go.uuid.v1 v1.2.0
	k8s.io/api v0.17.0
	k8s.io/apimachinery v0.17.0
	k8s.io/client-go v0.17.0
	k8s.io/klog v1.0.0
	sigs.k8s.io/kind v0.7.0
	sigs.k8s.io/yaml v1.1.1-0.20191128155103-745ef44e09d6 // branch master, with fixes in yaml.v2.2.7
)

// replace github.com/go-openapi/validate => ../go-openapi-validate
replace github.com/go-openapi/validate => github.com/flant/go-openapi-validate v0.19.4-0.20190926112101-38fbca4ac77f // branch: fix_in_body

replace github.com/onsi/ginkgo v1.11.0 => github.com/flant/ginkgo v1.11.1-0.20200206071017-2216da3b016c // fix_coverage_combining

//replace github.com/flant/libjq-go => ../libjq-go

// version from k8s.io/client-go
//replace k8s.io/klog => k8s.io/klog v0.0.0-20190306015804-8e90cee79f82
