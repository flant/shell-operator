# Example with conversion hooks

This is a simple example of `kubernetesCustomResourceConversion` binding. Read more information in [BINDING_CONVERSION.md](../../BINDING_CONVERSION.md).

The example contains one hook that is used for conversion of custom resource [CronTab](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/) on create or update (see also [105-crd-example](../105-crd-example/README.md). The `CronTab` has two versions: v1alpha1 and v1alpha2. The later one changes a type of .spec.crontab field from array to a string. The hook applies this change to resources with version v1alpha1.

## Run

### Generate certificates

An HTTP server for conversion webhook requires a certificate issued by the CA. For simplicity, this process is automated with `gen-certs.sh` script. Just run it:

```
./gen-certs.sh
```

> Note: `gen-certs.sh` requires [cfssl utility](https://github.com/cloudflare/cfssl/releases/latest).

### Build and install example

Build Docker image and use helm3 to install it:

```
docker build -t localhost:5000/shell-operator:example-210 .
docker push localhost:5000/shell-operator:example-210
helm upgrade --install \
    --namespace example-210 \
    --create-namespace \
    example-210 .
```

### See conversion hook in action

1. Create a CronTab with previous version v1alpha1:

```
$ cat crontab-v1alpha1.yaml
apiVersion: "stable.example.com/v1alpha1"
kind: CronTab
metadata:
  name: crontab-v1alpha1
  labels:
    heritage: example-210
spec:
  cron:
    - "*"
    - "*"
    - "*"
    - "*"
    - "*/5"
  imageName: repo.example.com/my-awesome-cron-image:v1

$ kubectl -n example-210 apply -f crontab-v1alpha1.yaml
crontab.stable.example.com/crontab-v1alpha1 created
```

2. Now get created resource back:

```
$ kubectl -n example-210 get ct/crontab-v1alpha1 -o yaml
  apiVersion: stable.example.com/v1alpha2
  kind: CronTab
  metadata:
    ...
  spec:
    cron: '* * * * */5'
    imageName: repo.example.com/my-awesome-cron-image:v1
```

Note that `apiVersion` is "stable.example.com/v1alpha2" and `spec.cron` is converted to a string.


### Cleanup

```
helm delete --namespace=example-210 example-210
kubectl delete ns example-210
kubectl delete crd crontabs.stable.example.com
```
