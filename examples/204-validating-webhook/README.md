# Example with validating hooks

This is a simple example of `kubernetesValidating` binding. Read more information in [VALIDATING.md](../../VALIDATING.md).

The example contains one hook that is used to validate custom resource [CronTab](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/) on create or update (see also [105-crd-example](../105-crd-example/README.md). The hook enforces a simple policy: it is forbidden to create a CronTab resource with "image" not from private repository "repo.example.com".

## Run

### Generate certificates

An HTTP server behind the ValidatingWebhookConfiguration requires a certificate issued by the CA. For simplicity, this process is automated with `gen-certs.sh` script. Just run it:

```
./gen-certs.sh
```

> Note: `gen-certs.sh` requires [cfssl utility](https://github.com/cloudflare/cfssl/releases/latest) and an [Approval Permission](https://kubernetes.io/docs/tasks/tls/managing-tls-in-a-cluster) in cluster.

### Build and install example

Build Docker image and use helm3 to install it:

```
docker build -t localhost:5000/shell-operator:example-204 .
docker push localhost:5000/shell-operator:example-204
helm upgrade --install \
    --namespace example-204 \
    --create-namespace \
    example-204 .
```

### See validating hook in action

1. Attempt to create a non valid CronTab:

```
$ kubectl -n example-204 apply -f crontab-non-valid.yaml
Error from server: error when creating "crontab-non-valid.yaml": admission webhook "private-repo-policy.example.com" denied the request: Only images from repo.example.com are allowed
$ kubectl -n example-204 get crontab
No resources found in example-204 namespace.
```

2. Create a valid CronTab:

```
$ kubectl -n example-204 apply -f crontab-valid.yaml
crontab.stable.example.com/crontab-valid created
$ kubectl -n example-204 get crontab
NAME            AGE
crontab-valid   9s
```

3. Change the "image" field in the valid CronTab:

```
$ kubectl -n example-204 patch crontab crontab-valid --type=merge -p '{"spec":{"image":"localhost:5000/crontab:latest"}}'
Error from server: admission webhook "private-repo-policy.example.com" denied the request: Only images from repo.example.com are allowed
```

### Cleanup

```
helm delete --namespace=example-204 example-204
kubectl delete ns example-204
kubectl delete validatingwebhookconfiguration/example-204
```
