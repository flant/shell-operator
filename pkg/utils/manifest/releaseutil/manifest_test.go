package releaseutil

import (
	"reflect"
	"testing"
)

const mockBigFile = `
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
`

var expectedManifests = map[string]string{
	"manifest-0": `# Source: big-chart/templates/poke-deployment.yaml`,
	"manifest-1": `apiVersion: apps/v1
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
        #- "while true; do echo 007 Buzzzzzzzz.; sleep 4; done"`,
	"manifest-2": `# Source: big-chart/templates/sleep-deploy-back.yaml
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
        command: ['/bin/ash', '-c', 'trap "echo Catch SIGINT, quit now" TERM INT; sleep 100000 & wait']`,
	"manifest-3": `# Source: big-chart/templates/sleep-deploy-front.yaml
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
        command: ['/bin/ash', '-c', 'trap "echo Catch SIGINT, quit now" TERM INT; sleep 100000 & wait']`,
}

func TestSplitManifest(t *testing.T) {
	manifests := SplitManifests(mockBigFile)
	if len(manifests) != 4 {
		t.Errorf("Expected 4 manifest, got %v", len(manifests))
	}

	if !reflect.DeepEqual(manifests, expectedManifests) {
		t.Errorf("Expected %v,\n got %v", expectedManifests, manifests)
	}
}
