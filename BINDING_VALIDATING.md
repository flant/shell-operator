# kubernetesValidating

This binding transforms a hook into a handler for ValidatingWebhookConfiguration. The Shell-operator creates ValidatingWebhookConfiguration, starts HTTPS server and use hooks to handle [AdmissionReview requests](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#request).

## Syntax

```yaml
configVersion: v1
onStartup: 10
kubernetes:
- name: myCrdObjects
  ...
kubernetesValidating:
- name: myCrdValidator
  includeSnapshotsFrom: ["myCrdObjects"]
  failurePolicy: Ignore | Fail (default)
  labelSelector:   # equivalent of objectSelector
    matchLabels:
      label1: value1
      ...
  namespace:
    labelSelector: # equivalent of namespaceSelector
      matchLabels:
        label1: value1
        ...
      matchExpressions:
      - key: environment
        operator: In
        values: ["prod","staging"]
  rules:
  - apiVersions:
    - v1
    apiGroups:
    - stable.example.com
    resources:
    - CronTab
    operations:
    - "*"
  - operations: ["CREATE", "UPDATE"]
    apiGroups: ["apps"]
    apiVersions: ["v1", "v1beta1"]
    resources: ["deployments", "replicasets"]
    scope: "Namespaced"
  sideEffects: None (default) | NoneOnDryRun
  timeoutSeconds: 2 (default is 10)
```

## Parameters

- `name` is a required parameter. It should be a domain with at least three segments separated by dots.

- `includeSnapshotsFrom` — an array of names of `kubernetes` bindings in a hook. When specified, a list of monitored objects from that bindings will be added to the binding context in a `snapshots` field.

- `labelSelector` — [standard](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#labelselector-v1-meta) selector of objects by labels (examples [of use](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels)). See [objectSelector](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#matching-requests-objectselector).

- `namespace.labelSelector` — this filter works like `labelSelector` but for namespaces. See [namespaceSelector](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#matching-requests-namespaceselector).

- `rules` — a list of rules used to determine if a request to the Kubernetes API server should be sent to the hook. See [Rules](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#matching-requests-rules).

- `failurePolicy` — defines how errors from the hook are handled. See [Failure policy](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#failure-policy). Default is `Fail`.

- `sideEffects` — determine whether the hook is `dryRun`-aware. See [side effects](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#side-effects) documentation. Default is `None`.

- `timeoutSeconds` — a seconds API server should wait for a hook to respond before treating the call as a failure. See [timeouts](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#timeouts). Default is 10 (seconds).

As you can see, it is the close copy of a [Webhook configuration](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#webhook-configuration). Differences are:
- `objectSelector` is a `labelSelector` as in the `kubernetes` binding.
- `namespaceSelector` is a `namespace.labelSelector` as in the `kubernetes` binding.
- `clientConfig` is managed by the Shell-operator. You should provide a Service for the Shell-operator HTTPS endpoint. See example [204-validating-webhook](./examples/204-validating-webhook) for possible solution.
- `matchPolicy` is always "Equivalent". See [Matching requests: matchPolicy](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#matching-requests-matchpolicy).
- there are additional fields `group` and `includeSnapshotsFrom` to include snapshots in the binding context.

## Example

```
configVersion: v1
kubernetesValidating:
- name: private-repo-policy.example.com
  rules:
  - apiGroups:   ["stable.example.com"]
    apiVersions: ["v1"]
    operations:  ["CREATE"]
    resources:   ["crontabs"]
    scope:       "Namespaced"
```

The Shell-operator will execute hook with this configuration on every creation of CronTab object.

See example [204-validating-webhook](./examples/204-validating-webhook).

## Hook input and output

The hook receives a binding context and should return response in `$VALIDATING_RESPONSE_PATH`.

$BINDING_CONTEXT_PATH file example:

```yaml
[{
"binding":"myCrdValidator",
"type":"Validating",
"snapshots": { ... }
# AdmissionReview object
"review": {
  "apiVersion": "admission.k8s.io/v1",
  "kind": "AdmissionReview",
  "request": {
    # Random uid uniquely identifying this admission call
    "uid": "705ab4f5-6393-11e8-b7cc-42010a800002",

    # Fully-qualified group/version/kind of the incoming object
    "kind": {"group":"autoscaling","version":"v1","kind":"Scale"},
    # Fully-qualified group/version/kind of the resource being modified
    "resource": {"group":"apps","version":"v1","resource":"deployments"},
    # subresource, if the request is to a subresource
    "subResource": "scale",

    # Fully-qualified group/version/kind of the incoming object in the original request to the API server.
    # This only differs from `kind` if the webhook specified `matchPolicy: Equivalent` and the
    # original request to the API server was converted to a version the webhook registered for.
    "requestKind": {"group":"autoscaling","version":"v1","kind":"Scale"},
    # Fully-qualified group/version/kind of the resource being modified in the original request to the API server.
    # This only differs from `resource` if the webhook specified `matchPolicy: Equivalent` and the
    # original request to the API server was converted to a version the webhook registered for.
    "requestResource": {"group":"apps","version":"v1","resource":"deployments"},
    # subresource, if the request is to a subresource
    # This only differs from `subResource` if the webhook specified `matchPolicy: Equivalent` and the
    # original request to the API server was converted to a version the webhook registered for.
    "requestSubResource": "scale",

    # Name of the resource being modified
    "name": "my-deployment",
    # Namespace of the resource being modified, if the resource is namespaced (or is a Namespace object)
    "namespace": "my-namespace",

    # operation can be CREATE, UPDATE, DELETE, or CONNECT
    "operation": "UPDATE",

    "userInfo": {
      # Username of the authenticated user making the request to the API server
      "username": "admin",
      # UID of the authenticated user making the request to the API server
      "uid": "014fbff9a07c",
      # Group memberships of the authenticated user making the request to the API server
      "groups": ["system:authenticated","my-admin-group"],
      # Arbitrary extra info associated with the user making the request to the API server.
      # This is populated by the API server authentication layer and should be included
      # if any SubjectAccessReview checks are performed by the webhook.
      "extra": {
        "some-key":["some-value1", "some-value2"]
      }
    },

    # object is the new object being admitted.
    # It is null for DELETE operations.
    "object": {"apiVersion":"autoscaling/v1","kind":"Scale",...},
    # oldObject is the existing object.
    # It is null for CREATE and CONNECT operations.
    "oldObject": {"apiVersion":"autoscaling/v1","kind":"Scale",...},
    # options contains the options for the operation being admitted, like meta.k8s.io/v1 CreateOptions, UpdateOptions, or DeleteOptions.
    # It is null for CONNECT operations.
    "options": {"apiVersion":"meta.k8s.io/v1","kind":"UpdateOptions",...},

    # dryRun indicates the API request is running in dry run mode and will not be persisted.
    # Webhooks with side effects should avoid actuating those side effects when dryRun is true.
    # See http://k8s.io/docs/reference/using-api/api-concepts/#make-a-dry-run-request for more details.
    "dryRun": false
  }
}
}]
```

Response example:
```
cat <<EOF >> $VALIDATING_RESPONSE_PATH
{"allowed": true}
EOF
```

Deny object creation and explain why:
```
cat <<EOF > $VALIDATING_RESPONSE_PATH
{"allowed": false, "message": "You cannot do this because it is Tuesday and your name starts with A"}
EOF
```

User will see an error message:

```
Error from server: admission webhook "policy.example.com" denied the request: You cannot do this because it is Tuesday and your name starts with A
```

Empty or invalid $VALIDATING_RESPONSE_PATH file is considered as `"allowed": false` with a short message about the problem and a more verbose error in the log.

## HTTP server and Kubernetes configuration

Shell-operator should create an HTTP endpoint with TLS support and register endpoints in the ValidatingWebhookConfiguration resource.

There should be a Service for shell-operator (see [Availability](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#availability)).

Command line options:

```
--validating-webhook-server-cert="/validating-certs/server.crt"
                             A path to a server certificate for ValidatingWebhook. Can be set with $VALIDATING_WEBHOOK_SERVER_CERT.
--validating-webhook-server-key="/validating-certs/server-key.pem"
                             A path to a server private key for ValidatingWebhook. Can be set with $VALIDATING_WEBHOOK_SERVER_KEY.
--validating-webhook-cluster-ca="/validating-certs/cluster-ca.pem"
                             A path to a cluster ca bundle for ValidatingWebhook. Can be set with $VALIDATING_WEBHOOK_CLUSTER_CA.
--validating-webhook-client-ca=VALIDATING-WEBHOOK-CLIENT-CA ...
                             A path to a server certificate for ValidatingWebhook. Can be set with $VALIDATING_WEBHOOK_CLIENT_CA.
--validating-webhook-service-name=VALIDATING-WEBHOOK-SERVICE-NAME ...
                             A name of a service in front of a shell-operator. Can be set with $VALIDATING_WEBHOOK_SERVICE_NAME.
```
