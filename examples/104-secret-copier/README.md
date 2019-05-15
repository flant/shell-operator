## Copying secrets to a created namespace

This example shows how to organize copying secrets to new namespaces after creation.

The source secrets having label 'secret-copier: yes' are copying from the 'default' namespace to the others namespaces in the Kubernetes cluster. This will occur in the following cases:
* after a secret with the lable 'secret-copier: yes' is created or changed in the 'default' namespace
* after a new namespace is created
* every night at 3 am (you can change it in the `hook/schedule_sync_secret` hook file)

When the secret having label 'secret-copier: yes' is deleted from the 'default' namespace, this secret is also deleted from other namespaces.

### Using

Build shell-operator image and push it to your Docker registry (replace the repository URL):
```shell
docker build -t "registry.mycompany.com/shell-operator:secret-copier" . &&
docker push registry.mycompany.com/shell-operator:secret-copier
```

Edit image in shell-operator-pod.yaml and apply manifests:

```shell
kubectl create ns secret-copier &&
kubectl -n secret-copier apply -f shell-operator-rbac.yaml &&
kubectl -n secret-copier apply -f shell-operator-pod.yaml
```

Create a secret in the 'default' namespaces, e.g. `myregistrysecret` secret with an access to your private Docker registry (replace necessary values):
```shell
kubectl create secret docker-registry myregistrysecret --docker-server=DOCKER_REGISTRY_SERVER --docker-username=DOCKER_USER \
--docker-password=DOCKER_PASSWORD --docker-email=DOCKER_EMAIL
```

Label the `myregistrysecret` with the 'secret-copier: yes' label:
```shell
kubectl label secret myregistrysecret secret-copier=yes
```

Check, that the `myregistrysecret` secret was copied to other namespaces:
```shell
kubectl get secret  --all-namespaces | grep myregistrysecret
```

There will be result depending on namespaces in the cluster, like this:
```
default           myregistrysecret                                 kubernetes.io/dockerconfigjson        1      91s
kube-node-lease   myregistrysecret                                 kubernetes.io/dockerconfigjson        1      35s
kube-public       myregistrysecret                                 kubernetes.io/dockerconfigjson        1      34s
kube-system       myregistrysecret                                 kubernetes.io/dockerconfigjson        1      33s
secret-copier     myregistrysecret                                 kubernetes.io/dockerconfigjson        1      33s
```

Create any namespace, for instance:

```
kubectl create ns foobar
```

See in logs that hook was run:

```
kubectl -n secret-copier logs po/shell-operator

...
2019-05-15T19:53:49Z INFO     : EVENT Kube event 'aef814be-b5f3-46e1-9241-c901b5ba03f8'
2019-05-15T19:53:49Z INFO     : QUEUE add TASK_HOOK_RUN@KUBE_EVENTS create_namespace
2019-05-15T19:53:49Z INFO     : TASK_RUN HookRun@KUBE_EVENTS create_namespace
2019-05-15T19:53:49Z INFO     : Running hook 'create_namespace' binding 'KUBE_EVENTS' ...
...
```

Get secret from the new namespace to check it was created by your hook:

```
kubectl get secret myregistrysecret -n foobar
```

### cleanup

Delete created kubernetes object:
```
kubectl delete secret myregistrysecret &&
sleep 10 &&
kubectl delete ns/secret-copier &&
kubectl delete clusterrolebindings secret-copier &&
kubectl delete serviceaccounts secret-copier-acc &&
kubectl delete clusterrole secret-copier
```

Delete docker image:
```
docker rmi registry.mycompany.com/shell-operator:secret-copier
```
