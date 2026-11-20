## monitor-configmap example

Example of monitoring ConfigMaps: the hook is run when a ConfigMap is added, deleted or its data is changed.

The hook monitors ConfigMaps in the `example-monitor-configmap` namespace only. It uses `jqFilter: ".data"`, so "Modified" events trigger the hook only when the data of a ConfigMap is changed. Changes in labels, annotations and other fields are ignored.

### Run

Build shell-operator image with custom script:

```
$ docker build -t "registry.mycompany.com/shell-operator:monitor-configmap" .
$ docker push registry.mycompany.com/shell-operator:monitor-configmap
```

Edit image in shell-operator-pod.yaml and apply manifests:

```
$ kubectl create ns example-monitor-configmap
$ kubectl -n example-monitor-configmap apply -f shell-operator-rbac.yaml
$ kubectl -n example-monitor-configmap apply -f shell-operator-pod.yaml
```

On startup, the hook is run with all existing ConfigMaps in the namespace ("Synchronization" binding context). Kubernetes creates the `kube-root-ca.crt` ConfigMap in every namespace, so it is always there.

Create a ConfigMap, change its data, add a label and delete the ConfigMap to trigger the hook:

```
$ kubectl -n example-monitor-configmap create configmap demo --from-literal=foo=bar
$ kubectl -n example-monitor-configmap patch configmap demo --type=merge -p '{"data":{"foo":"baz"}}'
$ kubectl -n example-monitor-configmap label configmap demo app=demo
$ kubectl -n example-monitor-configmap delete configmap demo
```

See in logs that hook was run. Shell-operator writes logs in JSON, lines with the hook's output are marked with `"output":"stdout"`:

```
$ kubectl -n example-monitor-configmap logs po/shell-operator | grep '"output":"stdout"' | jq -r .msg
ConfigMap 'kube-root-ca.crt' exists
ConfigMap 'demo' added with data {"foo":"bar"}
ConfigMap 'demo' data changed to {"foo":"baz"}
ConfigMap 'demo' deleted
```

Note that adding a label did not trigger the hook, because the data was not changed.

To monitor ConfigMaps in all namespaces, remove the `namespace` section from the hook configuration and use a ClusterRole and a ClusterRoleBinding instead of the Role and the RoleBinding (see [101-monitor-pods](../101-monitor-pods/shell-operator-rbac.yaml)).

### cleanup

```
$ kubectl delete ns/example-monitor-configmap
$ docker rmi registry.mycompany.com/shell-operator:monitor-configmap
```
