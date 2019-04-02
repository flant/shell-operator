# Shell-operator

Shell-operator is a tool for running event-driven scripts in a Kubernetes cluster.

* Simple configuration
* Fixed script execution order
* Run scripts on startup, on schedule or on Kubernetes events

## Quickstart

> You need to have a Kubernetes cluster, and the kubectl must be configured to communicate with your cluster.

To use Shell-operator you need to:
- build image with your hook or hooks (script)
- (optional) setup RBAC
- run Deployment with built image

### Build image with your hook

Hook is a script with the added events configuration code. Learn [more](HOOKS.md) about hooks.

Create project directory with the following Dockerfile, which use the [shell-operator](https://hub.docker.com/r/flant/shell-operator) image as FROM:
```
FROM: flant/shell-operator:..
ADD: hooks /hooks
```

In the project directory create `hooks/01-copy-secret` directory and the `hooks/01-copy-secret/hook.sh` file with the following content:

```
#!/usr/bin/env bash

if [[ $1 == "--config" ]] ; then
  cat <<EOF
{
  "onKubernetesEvent": [
    {
      "name": "Registry secret copier",
      "kind": "namespace",
      "event": ["add"],
      },
  ]
}
EOF
  exit 0
fi

#TODO


echo "Finish."
```

Build image and push it to the Docker registry.

### Use image in your cluster

To use the built image in your Kubernetes cluster you need to create a Deployment.
Somewhere out of the project directory create a `shell-operator.yml` file describing deployment with the following content:
```
#TODO

```

Start shell-operator by applying `shell-operator.yml`:
```
kubectl apply shell-operator.yml
```

## Examples

## License

Apache License 2.0, see [LICENSE](LICENSE).
