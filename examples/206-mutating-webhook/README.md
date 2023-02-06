# Example with mutating hook

This is a simple example of `kubernetesMutating` binding. Read more information in [BINDING_MUTATING.md](../../BINDING_MUTATING.md).

The example contains one hook that is used to mutate custom resource [CronTab](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/) on create or update (see also [105-crd-example](../105-crd-example/README.md). The hook just updates a `spec.replicas` field to value 333.

## Run

### Generate certificates

An HTTP server behind the MutatingWebhookConfiguration requires a certificate issued by the CA. For simplicity, this process is automated with `gen-certs.sh` script. Just run it:

```
./gen-certs.sh
```

> Note: `gen-certs.sh` requires [cfssl utility](https://github.com/cloudflare/cfssl/releases/latest).

### Build and install example

Build Docker image and use helm3 to install it:

```
docker build -t localhost:5000/shell-operator:example-206 .
docker push localhost:5000/shell-operator:example-206
helm upgrade --install \
    --namespace example-206 \
    --create-namespace \
    example-206 .
```

### See mutating hook in action

Create a CronTab resource:

```
$ kubectl -n example-206 apply -f crontab-valid.yaml
crontab.stable.example.com/crontab-valid created
```

Check CronTab manifest:

```
$ kubectl -n example-206 get crontab -o yaml
...
apiVersion: stable.example.com/v1
kind: CronTab
metadata:
  name: crontab-valid
  ...
spec:
  cronSpec: '* * * * */5'    <--- It is from the YAML file.
  image: repo.example.com/my-awesome-cron-image:v1    <--- It is from the YAML file.
  replicas: 333  <--- This one is set by the hook.
...
```

### Cleanup

```
helm delete --namespace=example-206 example-206
kubectl delete ns example-206
kubectl delete mutatingwebhookconfiguration/example-206
```
