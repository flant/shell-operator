**Story**: I want to use a shell-operator's hook as a ValidatingWebhook handler.

**Implementation**:
- New section in hook configuration.
- Another HTTP server in shell-operator with TLS enabled to receive AdmissionReview objects
- Extend binding context.
- Support validating handlers in `frameworks/shell`.

## Configuration

A proposed configuration is a close copy of a `webhooks` field from [ValidatingWebhookConfiguration](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#configure-admission-webhooks-on-the-fly). One shell-operator hook can have many webhooks with different rules and selectors.

Differences are:
- `objectSelector` is a `labelSelector` as in `kubernetes` binding.
- `namespaceSelector` is a `namespace.labelSelector` as in `kubernetes` binding.
- `clientConfig` is managed by shell-operator.
- `matchPolicy` is always "Equivalent". See https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#intercepting-all-versions-of-an-object
- additional fields `group` and `includeSnapshotsFrom` to include snapshots in the binding context.

Hook configuration example:

```yaml
configVersion: v1
onStartup: 10
kubernetes:
- name: myCrdObjects
   kind: MyCrd
   jqFilter: ".spec"
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
    - crd-domain.io
    resources:
    - MyCustomResource
    operations:
    - *
  - operations: ["CREATE", "UPDATE"]
    apiGroups: ["apps"]
    apiVersions: ["v1", "v1beta1"]
    resources: ["deployments", "replicasets"]
    scope: "Namespaced"
  sideEffects: None (default) | NoneOnDryRun
  timeoutSeconds: 2 (default is 10)
```

## Hook input and output

The hook receives a binding context and should return response in `$VALIDATION_RESPONSE_PATH`.

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

Customize error message:
```
cat <<EOF >> $VALIDATING_RESPONSE_PATH
{"allowed": false,
"status": {
      "code": 403,
      "message": "You cannot do this because it is Tuesday and your name starts with A"
    }
 }
EOF
```


## HTTP server and Kubernetes configuration

Shell-operator should create an HTTP endpoint with TLS support and register endpoints in the ValidatingWebhookConfiguration resource.

There should be a Service for shell-operator (see [Availability](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#availability)).

New options:

```
  --validating-webhook-server-cert="/validating-certs/server.crt"
                                 A path to a server certificate for ValidatingWebhook. Can be set with $VALIDATING_WEBHOOK_SERVER_CERT.
  --validating-webhook-server-key="/validating-certs/server-key.pem"
                                 A path to a server private key for ValidatingWebhook. Can be set with $VALIDATING_WEBHOOK_SERVER_KEY.
  --validating-webhook-CLUSTER-ca="/validating-certs/cluster-ca.pem"
                                 A path to a cluster ca bundle for ValidatingWebhook. Can be set with $VALIDATING_WEBHOOK_CLUSTER_CA.
  --validating-webhook-client-ca=VALIDATING-WEBHOOK-CLIENT-CA ...
                                 A path to a server certificate for ValidatingWebhook. Can be set with $VALIDATING_WEBHOOK_CLIENT_CA.
  --validating-webhook-service-name=VALIDATING-WEBHOOK-SERVICE-NAME ...
                                 A name of a service in front of a shell-operator. Can be set with $VALIDATING_WEBHOOK_SERVICE_NAME.
```

## Hook execution

1. The shell-operator starts WebhookManager service at the start after starting informers and successful Synchronization.

2. WebhookManager service starts the HTTP server.

3.  WebhookManager service recreates `ValidatingWebhookConfiguration` resource. Items in `kubernetesValidating` in hooks are expanded with `clientConfig` field that contains a `path` field.

3.1. There is only one `ValidatingWebhookConfiguration` resource for one shell-operator instance.

4. The `path` field allows identifying which hook to execute upon receiving AdmissionReview. It consists of a safe hook name and a safe webhook name to identify hook script and a webhook.

5. WebhookManager service has a channel to send hook execution events to HookManager. HookManager should execute hooks without queuing.

6. Hook gets binding context as input and should write a response to a $VALIDATING_RESPONSE_PATH file. Binding context contains `binding: name-from-webhook`, `type: Validating`, `snapshots: ` and `review: <AdmissionReview json>` fields.

7. Empty or invalid $VALIDATING_RESPONSE_PATH file is considered as `"allow": false` with a message about the situation.

```
$ kubectl -n shell-test apply -f crontab-obj.yaml
Error from server: error when creating "crontab-obj.yaml": admission webhook "root-policy.example.com" denied the request: Shell-operator hook 'validating-hook.sh' error: invalid response.
```

More verbose error is available in logs.

8. Framework tries these handlers for `type: Validating`:

```
__on_kubernetes_validating::${BINDING_CONTEXT_CURRENT_BINDING}::validating __on_kubernetes_validating::${BINDING_CONTEXT_CURRENT_BINDING}
__main__
```

9. Binding context is included snapshots as stated in `group` and `includeSnapshotsFrom` fields in `kubernetesValidating` items.