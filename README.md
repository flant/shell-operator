
<p align="center">
<img src="docs/src/image/shell-operator-small-logo.png" alt="shell-operator logo" />
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
- __Kubernetes object events__: hook can be triggered by `add`, `update` or `delete` events. **[Learn more](docs/src/HOOKS.md) about hooks.**
- __Object selector and properties filter__: shell-operator can monitor a particular set of objects and detect changes in their properties.
- __Simple configuration__: hook binding definition is a JSON or YAML document on script's stdout.
- __Validating webhook machinery__: hook can handle validating for Kubernetes resources.
- __Conversion webhook machinery__: hook can handle version conversion for Kubernetes resources.

# Documentation

Please see the [docs](https://flant.github.io/shell-operator/) for more in-depth information and supported features.

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
