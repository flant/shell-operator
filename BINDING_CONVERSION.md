# kubernetesCustomResourceConversion

This binding transforms a hook into a handler for conversions defined in CustomResourceDefinition. The Shell-operator updates a CRD with .spec.conversion, starts HTTPS server, and runs hooks to handle [ConversionReview requests](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/#conversionreview-request-0).

> Note: shell-operator use `apiextensions.k8s.io/v1`, so Kubernetes 1.16+ is required.

## Syntax

```yaml
configVersion: v1
onStartup: 10
kubernetes:
- name: additionalObjects
  ...
kubernetesCustomResourceConversion:
- name: alpha1_to_alpha2
  # Include snapshots by binding names.
  includeSnapshotsFrom: ["additionalObjects"]
  # Or use group name to include all snapshots in a group.
  group: "group name"
  # A CRD name.
  crdName: crontabs.stable.example.com
  # An array of conversions supported by this hook.
  conversion:
  - fromVersion: stable.example.com/v1alpha1
    toVersion: stable.example.com/v1alpha2
```

## Parameters

- `name` — a required parameter. It is used to distinguish between multiple schedules during runtime. For more information see [binding context](HOOKS.md#binding-context).

- `includeSnapshotsFrom` — an array of names of `kubernetes` bindings in a hook. When specified, a list of monitored objects from these bindings will be added to the binding context in the `snapshots` field.

- `group` — a key to include snapshots from a group of `schedule` and `kubernetes` bindings. See [grouping](HOOKS.md#an-example-of-a-binding-context-with-group).

- `crdName` — a required name of a CRD.

- `conversions` — a required list of conversion rules. These rules are used to determine if a custom resource in ConversionReview can be converted by the hook.

    - `fromVersion` — a version of a custom resource that hook can convert.

    - `toVersion` — a version of a custom resource that hook can produce.


## Example

```
configVersion: v1
kubernetesCustomResourceConversion:
- name: conversions
  crdName: crontabs.stable.example.com
  conversions:
  - fromVersion: unstable.crontab.io/v1beta1
    toVersion: stable.example.com/v1beta1
  - fromVersion: stable.example.com/v1beta1
    toVersion: stable.example.com/v1beta2
  - fromVersion: v1beta2
    toVersion: v1
```

The Shell-operator will execute this hook to convert custom resources 'crontabs.stable.example.com' from unstable.crontab.io/v1beta1 to stable.example.com/v1beta1, from stable.example.com/v1beta1 to stable.example.com/v1beta2, from unstable.crontab.io/v1beta1 to stable.example.com/v1 and so on.

See example [210-conversion-webhook](./examples/210-conversion-webhook).

## Hook input and output

> Note that the `group` parameter is only for including snapshots. `kubernetesCustomResourceConversion` hook is never executed on `schedule` or `kubernetes` events with binding context with `"type":"Group"`.

The hook receives a binding context and should return response in `$CONVERSION_RESPONSE_PATH`.

$BINDING_CONTEXT_PATH file example:

```yaml
[{
# Name as defined in binding configuration.
"binding": "alpha1_to_alpha2",
# type "Conversion" to distinguish from other events.
"type": "Conversion",
# Snapshots as defined by includeSnapshotsFrom or group.
"snapshots": { ... }
# fromVersion and toVersion as defined in a conversion rule.
"fromVersion": "unstable.crontab.io/v1beta1",
"toVersion": "stable.example.com/v1beta1",
# ConversionReview object.
"review": {
  "apiVersion": "apiextensions.k8s.io/v1",
  "kind": "ConversionReview",
  "request": {
    "desiredAPIVersion": "stable.example.com/v1beta1",
    "objects": [
      {
        # A source version.
        "apiVersion": "unstable.crontab.io/v1beta1",
        "kind": "CronTab",
        "metadata": {
          "name": "crontab-v1alpha1",
          "namespace": "example-210",
          ...
        },
        "spec": {
          "cron": [
            "*",
            "*",
            "*",
            "*",
            "*/5"
          ],
          "imageName": [
            "repo.example.com/my-awesome-cron-image:v1"
          ]
        }
      }
    ],
    "uid": "42f90c87-87f5-4686-8109-eba065c7fa6e"
  }
}
}]
```

Response example:
```
    cat <<EOF >$CONVERSION_RESPONSE_PATH
{"convertedObjects": [{
# A converted version.
"apiVersion": "stable.example.com/v1beta1",
"kind": "CronTab",
"metadata": {
  "name": "crontab-v1alpha1",
  "namespace": "example-210",
  ...
},
"spec": {
  "cron": [
    "*",
    "*",
    "*",
    "*",
    "*/5"
  ],
  "imageName": [
    "repo.example.com/my-awesome-cron-image:v1"
  ]
}
}]}
EOF
```

Return a message if something goes wrong:
```
cat <<EOF >$CONVERSION_RESPONSE_PATH
{"failedMessage":"Conversion of crontabs.stable.example.com is failed"}
EOF
```

User will see an error message:

```
Error from server: conversion webhook for unstable.crontab.io/v1beta1, Kind=CronTab failed: Conversion of crontabs.stable.example.com is failed
```

Empty or invalid $CONVERSION_RESPONSE_PATH file is considered as a fail with a short message about the problem and a more verbose error in the log.

## HTTP server and Kubernetes configuration

Shell-operator should create an HTTP endpoint with TLS support and register an endpoint in the CustomResourceDefinition resource.

There should be a Service for shell-operator (see [Service Reference](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/#service-reference)).

There are command line options and corresponding environment variables to setup TLS certificates and a service name:

```
  --conversion-webhook-service-name="shell-operator-conversion-svc"
                                 A name of a service for clientConfig in CRD. Can be set with
                                 $CONVERSION_WEBHOOK_SERVICE_NAME.
  --conversion-webhook-server-cert="/conversion-certs/tls.crt"
                                 A path to a server certificate for clientConfig in CRD. Can be set with
                                 $CONVERSION_WEBHOOK_SERVER_CERT.
  --conversion-webhook-server-key="/conversion-certs/tls.key"
                                 A path to a server private key for clientConfig in CRD. Can be set with
                                 $CONVERSION_WEBHOOK_SERVER_KEY.
  --conversion-webhook-ca="/conversion-certs/ca.crt"
                                 A path to a ca certificate for clientConfig in CRD. Can be set with
                                 $CONVERSION_WEBHOOK_CA.
  --conversion-webhook-client-ca=CONVERSION-WEBHOOK-CLIENT-CA ...
                                 A path to a server certificate for CRD.spec.conversion.webhook. Can be set
                                 with $CONVERSION_WEBHOOK_CLIENT_CA.
```
