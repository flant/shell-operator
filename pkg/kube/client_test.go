package kube

import (
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

func Test_Discover(t *testing.T) {
	// Test works only with real cluster.
	t.SkipNow()

	err := Init(InitOptions{})
	if err != nil {
		t.Error(err)
	}

	lists, err := Kubernetes.Discovery().ServerPreferredResources()
	if err != nil {
		t.Error(err)
	}

	fmt.Printf("Got %d lists\n", len(lists))

	for _, list := range lists {
		fmt.Printf("\n\n")
		fmt.Printf("%s has %d resources\n", list.GroupVersion, len(list.APIResources))

		if len(list.APIResources) == 0 {
			continue
		}

		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			t.Error(err)
			continue
		}

		for _, resource := range list.APIResources {
			gvr := schema.GroupVersionResource{
				Resource: resource.Name,
				Group:    gv.Group,
				Version:  gv.Version,
			}

			fmt.Printf("%30s %30s %30s %d:%+v\n",
				gvr.GroupVersion().String(),
				resource.Kind,
				fmt.Sprintf("%+v", append([]string{resource.Name}, resource.ShortNames...)),
				len(resource.Verbs), resource.Verbs)
		}
	}
}
