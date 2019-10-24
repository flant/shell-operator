module github.com/flant/shell-operator

go 1.12

require (
	github.com/go-openapi/errors v0.19.2
	github.com/go-openapi/spec v0.19.3
	github.com/go-openapi/strfmt v0.19.2
	github.com/go-openapi/swag v0.19.5
	github.com/go-openapi/validate v0.19.3
	github.com/hashicorp/go-multierror v1.0.0
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/kennygrant/sanitize v1.2.4
	github.com/prometheus/client_golang v1.0.0
	github.com/sirupsen/logrus v1.4.2
	github.com/stretchr/testify v1.4.0
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45 // indirect
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4 // indirect
	google.golang.org/appengine v1.5.0 // indirect
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/robfig/cron.v2 v2.0.0-20150107220207-be2e0b0deed5
	gopkg.in/satori/go.uuid.v1 v1.2.0
	gopkg.in/yaml.v2 v2.2.4
	k8s.io/api v0.0.0-20190409092523-d687e77c8ae9 // indirect
	k8s.io/apimachinery v0.0.0-20190409092423-760d1845f48b
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/utils v0.0.0-20190308190857-21c4ce38f2a7 // indirect
	sigs.k8s.io/kind v0.0.0-20191024075107-85a46de11816
)

// replace github.com/go-openapi/validate => ../go-openapi-validate
replace github.com/go-openapi/validate => github.com/flant/go-openapi-validate v0.19.4-0.20190926112101-38fbca4ac77f // branch: fix_in_body
