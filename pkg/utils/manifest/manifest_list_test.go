package manifest

import (
	"testing"

	. "github.com/onsi/gomega"
)

const renderOutput = `
---
# Source: big-chart/templates/poke-deployment.yaml
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: big-chart-buzz
spec:
  replicas: 1
  selector:
    matchLabels:
      app: big-chart
      component: buzz
  template:
    metadata:
      labels:
        app: big-chart
        component: buzz
    spec:
      containers:
      - name: big-chart-buzz
        image: alpine:3.9
        command: ['/bin/ash', '-c', 'trap "echo Catch SIGINT, quit now" TERM INT; sleep 100000 & wait']
        #command: ["/bin/ash"]
        #args:
        #- "-c"
        #- "while true; do echo 007 Buzzzzzzzz.; sleep 4; done"

---
# Source: big-chart/templates/sleep-deploy-back.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: big-chart-back
spec:
  selector:
    matchLabels:
      app: big-chart
      component: back
  replicas: 3
  template:
    metadata:
      name: big-chart-back
      labels:
        app: big-chart
        component: back
    spec:
      containers:
      - name: big-chart-back
        image: alpine:3.9
        imagePullPolicy: Always
        command: ['/bin/ash', '-c', 'trap "echo Catch SIGINT, quit now" TERM INT; sleep 100000 & wait']

---
# Source: big-chart/templates/sleep-deploy-front.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: big-chart-front
spec:
  selector:
    matchLabels:
      app: big-chart
      component: front
  replicas: 5
  template:
    metadata:
      name: big-chart-front
      labels:
        app: big-chart
        component: front
    spec:
      containers:
      - name: big-chart-front
        image: alpine:3.9
        imagePullPolicy: Always
        command: ['/bin/ash', '-c', 'trap "echo Catch SIGINT, quit now" TERM INT; sleep 100000 & wait']
---
# Source: no-source
apiVersion: apps/v1
metadata:
  name: big-chart-front
spec:
  selector:
    matchLabels:
      app: big-chart
      component: front
`

func Test_RenderedChart_To_ManifestList(t *testing.T) {
	g := NewWithT(t)
	manifests, err := ListFromYamlDocs(renderOutput)

	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(manifests).To(HaveLen(3), "GetManifestListFromYamlDocuments should return only templates with all basic fields: kind, apiVersion and metadata.name")
}
