module github.com/flant/shell-operator

go 1.16

require (
	github.com/flant/kube-client v0.0.7-0.20221207130940-42768d5a4e2c
	github.com/flant/libjq-go v1.6.2-0.20200616114952-907039e8a02a // branch: master
	github.com/go-chi/chi/v5 v5.0.7
	github.com/go-openapi/spec v0.19.8
	github.com/go-openapi/strfmt v0.19.5
	github.com/go-openapi/swag v0.19.14
	github.com/go-openapi/validate v0.19.12
	github.com/hashicorp/go-multierror v1.1.1
	github.com/kennygrant/sanitize v1.2.4
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.20.1
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.12.1
	github.com/sirupsen/logrus v1.8.1
	github.com/stretchr/testify v1.8.0
	golang.org/x/time v0.0.0-20220210224613-90d013bbcef8
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/robfig/cron.v2 v2.0.0-20150107220207-be2e0b0deed5
	gopkg.in/satori/go.uuid.v1 v1.2.0
	gopkg.in/yaml.v3 v3.0.1
	k8s.io/api v0.25.4
	k8s.io/apiextensions-apiserver v0.25.4
	k8s.io/apimachinery v0.25.4
	k8s.io/client-go v0.25.4
	sigs.k8s.io/yaml v1.2.0
)

// Remove 'in body' from errors, fix for Go 1.16 (https://github.com/go-openapi/validate/pull/138).
replace github.com/go-openapi/validate => github.com/flant/go-openapi-validate v0.19.12-flant.0

require github.com/matttproud/golang_protobuf_extensions v1.0.2-0.20181231171920-c182affec369 // indirect
