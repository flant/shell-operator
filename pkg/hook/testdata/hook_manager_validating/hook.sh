#!/usr/bin/env bash

if [[ $1 == "--config" ]] ; then
    cat <<EOF
configVersion: v1
onStartup: 10
kubernetes:
- name: pods
  group: pods
  kind: Pod
  labelSelector:
    matchLabels:
      foo: bar
kubernetesValidating:
- name: test-policy.example.com
  labelSelector:
    matchLabels:
      foo: bar
  rules:
  - apiGroups:   [""]
    apiVersions: ["v1"]
    operations:  ["CREATE"]
    resources:   ["pods"]
    scope:       "Namespaced"
  - apiGroups:   [""]
    apiVersions: ["v1"]
    operations:  ["CREATE"]
    resources:   ["pods"]
    scope:       "Namespaced"
  - apiGroups:   [""]
    apiVersions: ["v1"]
    operations:  ["CREATE"]
    resources:   ["pods"]
    scope:       "Namespaced"
  - apiGroups:   [""]
    apiVersions: ["v1"]
    operations:  ["CREATE"]
    resources:   ["pods"]
    scope:       "Namespaced"
  - apiGroups:   ["stable.example.com"]
    apiVersions: ["v1"]
    operations:  ["CREATE"]
    resources:   ["crontabs"]
    scope:       "Namespaced"
  timeoutSeconds: 15
EOF

else
  exit 0
fi