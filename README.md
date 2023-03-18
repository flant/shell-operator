<p align="center">
<img src="docs/shell-operator-small-logo.png" alt="shell-operator logo" />
</p>

<p align="center">
<a href="https://hub.docker.com/r/flant/shell-operator"><img src="https://img.shields.io/docker/pulls/flant/shell-operator.svg?logo=docker" alt="docker pull flant/shell-operator"/></a>
 <a href="https://github.com/flant/shell-operator/discussions"><img src="https://img.shields.io/badge/GitHub-discussions-brightgreen" alt="GH Discussions"/></a>
</p>

**Shell-operator** is a tool for running event-driven scripts in a Kubernetes cluster.

This operator is not an operator for a _particular software product_ such as `prometheus-operator` or `kafka-operator`. Shell-operator provides an integration layer between Kubernetes cluster events and shell scripts by treating scripts as hooks triggered by events. Think of it as an `operator-sdk` but for scripts.

Shell-operator is used as a base for more advanced [addon-operator](https://github.com/flant/addon-operator) that supports Helm charts and value storages.

Shell-operator provides:

- __Ease of management of a Kubernetes cluster__: use the tools that Ops are familiar with. It can be bash, python, kubectl, etc.
- __Kubernetes object events__: hook can be triggered by `add`, `update` or `delete` events. **[Learn more](HOOKS.md) about hooks.**
- __Object selector and properties filter__: shell-operator can monitor a particular set of objects and detect changes in their properties.
- __Simple configuration__: hook binding definition is a JSON or YAML document on script's stdout.
- __Validating webhook machinery__: hook can handle validating for Kubernetes resources.
- __Conversion webhook machinery__: hook can handle version conversion for Kubernetes resources.

**Contents**:
* [Quickstart](#quickstart)
  * [Build an image with your hooks](#build-an-image-with-your-hooks)
  * [Create RBAC objects](#create-rbac-objects)
  * [Install shell-operator in a cluster](#install-shell-operator-in-a-cluster)
  * [It all comes together](#it-all-comes-together)
* [Hook binding types](#hook-binding-types)
  * [`kubernetes`](#kubernetes)
  * [`onStartup`](#onstartup)
  * [`schedule`](#schedule)
* [Prometheus target](#prometheus-target)
* [Examples and notable users](#examples-and-notable-users)
* [Articles & talks](#articles--talks)
* [Community](#community)
* [License](#license)

# Quickstart

> You need to have a Kubernetes cluster, and the `kubectl` must be configured to communicate with your cluster.

The simplest setup of shell-operator in your cluster consists of these steps:

- build an image with your hooks (scripts)
- create necessary RBAC objects (for `kubernetes` bindings)
- run Pod or Deployment with the built image

For more configuration options see [RUNNING](RUNNING.md).

## Build an image with your hooks

A hook is a script that, when executed with `--config` option, outputs configuration to stdout in YAML or JSON format. [Learn more](HOOKS.md) about hooks.

Let's create a small operator that will watch for all Pods in all Namespaces and simply log the name of a new Pod.

`kubernetes` binding is used to tell shell-operator about objects that we want to watch. Create the `pods-hook.sh` file with the following content:

```bash
#!/usr/bin/env bash

if [[ $1 == "--config" ]] ; then
  cat <<EOF
configVersion: v1
kubernetes:
- apiVersion: v1
  kind: Pod
  executeHookOnEvent: ["Added"]
EOF
else
  podName=$(jq -r .[0].object.metadata.name $BINDING_CONTEXT_PATH)
  echo "Pod '${podName}' added"
fi
```

Make the `pods-hook.sh` executable:

```shell
chmod +x pods-hook.sh
```

You can use a prebuilt image [ghcr.io/flant/shell-operator:latest](https://github.com/flant/shell-operator/pkgs/container/shell-operator) with `bash`, `kubectl`, `jq` and `shell-operator` binaries to build you own image. You just need to `ADD` your hook into `/hooks` directory in the `Dockerfile`.

Create the following `Dockerfile` in the directory where you created the `pods-hook.sh` file:

```dockerfile
FROM ghcr.io/flant/shell-operator:latest
ADD pods-hook.sh /hooks
```

Build an image (change image tag according to your Docker registry):

```shell
docker build -t "registry.mycompany.com/shell-operator:monitor-pods" .
```

Push image to the Docker registry accessible by the Kubernetes cluster:

```shell
docker push registry.mycompany.com/shell-operator:monitor-pods
```

## Create RBAC objects

We need to watch for Pods in all Namespaces. That means that we need specific RBAC definitions for shell-operator:

```shell
kubectl create namespace example-monitor-pods
kubectl create serviceaccount monitor-pods-acc --namespace example-monitor-pods
kubectl create clusterrole monitor-pods --verb=get,watch,list --resource=pods
kubectl create clusterrolebinding monitor-pods --clusterrole=monitor-pods --serviceaccount=example-monitor-pods:monitor-pods-acc
```

## Install shell-operator in a cluster

Shell-operator can be deployed as a Pod. Put this manifest into the `shell-operator-pod.yaml` file:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: shell-operator
spec:
  containers:
  - name: shell-operator
    image: registry.mycompany.com/shell-operator:monitor-pods
    imagePullPolicy: Always
  serviceAccountName: monitor-pods-acc
```

Start shell-operator by applying a `shell-operator-pod.yaml` file:

```shell
kubectl -n example-monitor-pods apply -f shell-operator-pod.yaml
```

## It all comes together

Let's deploy a [kubernetes-dashboard](https://kubernetes.io/docs/tasks/access-application-cluster/web-ui-dashboard/) to trigger  `kubernetes` binding defined in our hook:

```shell
kubectl apply -f https://raw.githubusercontent.com/kubernetes/dashboard/master/aio/deploy/recommended.yaml
```

Now run `kubectl -n example-monitor-pods logs po/shell-operator` and observe that the hook will print dashboard pod name:

```plain
...
INFO[0027] queue task HookRun:main                       operator.component=handleEvents queue=main
INFO[0030] Execute hook                                  binding=kubernetes hook=pods-hook.sh operator.component=taskRunner queue=main task=HookRun
INFO[0030] Pod 'kubernetes-dashboard-775dd7f59c-hr7kj' added  binding=kubernetes hook=pods-hook.sh output=stdout queue=main task=HookRun
INFO[0030] Hook executed successfully                    binding=kubernetes hook=pods-hook.sh operator.component=taskRunner queue=main task=HookRun
...
```

> *Note:* hook output is logged with output=stdout label.

To clean up a cluster, delete namespace and RBAC objects:

```shell
kubectl delete ns example-monitor-pods
kubectl delete clusterrole monitor-pods
kubectl delete clusterrolebinding monitor-pods
```

This example is also available in /examples: [monitor-pods](examples/101-monitor-pods).

# Hook binding types

Every hook should respond with JSON or YAML configuration of bindings when executed with `--config` flag.

## kubernetes

This binding defines a subset of Kubernetes objects that shell-operator will monitor and a [jq](https://github.com/stedolan/jq/) expression to filter their properties. Read more about `onKubernetesEvent` bindings [here](HOOKS.md#kubernetes).

Example of YAML output from `hook --config`:

```yaml
configVersion: v1
kubernetes:
- name: execute_on_changes_of_namespace_labels
  kind: Namespace
  executeHookOnEvent: ["Modified"]
  jqFilter: ".metadata.labels"
```

> Note: it is possible to watch Custom Defined Resources, just use proper values for `apiVersion` and `kind` fields.

> Note: return configuration as JSON is also possible as JSON is a subset of YAML.

## onStartup

This binding has only one parameter: order of execution. Hooks are loaded at the start and then hooks with onStartup binding are executed in the order defined by parameter. Read more about `onStartup` bindings [here](HOOKS.md#onstartup).

Example `hook --config`:

```yaml
configVersion: v1
onStartup: 10
```

## schedule

This binding is used to execute hooks periodically. A schedule can be defined with a granularity of seconds. Read more about `schedule` bindings [here](HOOKS.md#schedule).

Example `hook --config` with 2 schedules:

```yaml
configVersion: v1
schedule:
- name: "every 10 min"
  crontab: "*/10 * * * *"
  allowFailure: true
- name: "Every Monday at 8:05"
  crontab: "5 8 * * 1"
  queue: mondays
```

# Prometheus target

Shell-operator provides a `/metrics` endpoint. More on this in [METRICS](METRICS.md) document.

# Examples and notable users

More examples of how you can use shell-operator are available in the [examples](examples/) directory.

Prominent shell-operator use cases include:

* [Deckhouse](https://deckhouse.io/) Kubernetes platform where both projects, shell-operator and addon-operator, are used as the core technology to configure & extend K8s features;
* KubeSphere Kubernetes platform's [installer](https://github.com/kubesphere/ks-installer);
* [Kafka DevOps solution](https://github.com/confluentinc/streaming-ops) from Confluent.

Please find out & share more examples in [Show & tell discussions](https://github.com/flant/shell-operator/discussions/categories/show-and-tell).

# Articles & talks

Shell-operator has been presented during KubeCon + CloudNativeCon Europe 2020 Virtual (Aug'20). Here is the talk called "Go? Bash! Meet the shell-operator":

* [YouTube video](https://www.youtube.com/watch?v=we0s4ETUBLc);
* [text summary](https://medium.com/flant-com/meet-the-shell-operator-kubecon-36c14ba2f8fe);
* [slides](https://speakerdeck.com/flant/go-bash-meet-the-shell-operator).

Official publications on shell-operator:
* "[shell-operator v1.0.0: the long-awaited release of our tool to create Kubernetes operators](https://blog.deckhouse.io/shell-operator-v1-0-0-the-long-awaited-release-of-our-tool-to-create-kubernetes-operators-b20fc0bbca9f?source=friends_link&sk=4d5f991eef62ad22222c5a725712ccdd)" (Apr'21);
* "[shell-operator & addon-operator news: hooks as admission webhooks, Helm 3, OpenAPI, Go hooks, and more!](https://blog.deckhouse.io/shell-operator-addon-operator-news-hooks-as-admission-webhooks-helm-3-openapi-go-hooks-and-369df9b4af08?source=friends_link&sk=142aec38bdcdbca73868eb4cc0b85483)" (Feb'21);
* "[Kubernetes operators made easy with shell-operator: project status & news](https://blog.deckhouse.io/shell-operator-for-kubernetes-update-2f1f9f9ebfb1)" (Jul'20);
* "[Announcing shell-operator to simplify creating of Kubernetes operators](https://blog.deckhouse.io/kubernetes-shell-operator-76c596b42f23)" (May'19).

Other languages:
* Chinese: "[介绍一个不太小的工具：Shell Operator](https://blog.fleeto.us/post/shell-operator/)"; "[使用shell-operator实现Operator](https://cloud.tencent.com/developer/article/1701733)";
* Dutch: "[Een operator om te automatiseren – Hoe pak je dat aan?](https://www.hcs-company.com/blog/operator-automatiseren-namespace-openshift)";
* Russian: "[shell-operator v1.0.0: долгожданный релиз нашего проекта для Kubernetes-операторов](https://habr.com/ru/company/flant/blog/551456/)"; "[Представляем shell-operator: создавать операторы для Kubernetes стало ещё проще](https://habr.com/ru/company/flant/blog/447442/)".

# Community

Please feel free to reach developers/maintainers and users via [GitHub Discussions](https://github.com/flant/shell-operator/discussions) for any questions regarding shell-operator.

You're also welcome to follow [@flant_com](https://twitter.com/flant_com) to stay informed about all our Open Source initiatives.

# License

Apache License 2.0, see [LICENSE](LICENSE).
