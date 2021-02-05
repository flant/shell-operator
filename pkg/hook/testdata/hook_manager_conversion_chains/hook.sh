#!/usr/bin/env bash

if [[ $1 == "--config" ]] ; then
    cat <<EOF
configVersion: v1
onStartup: 10
kubernetesCustomResourceConversion:
- name: up_conversions
  crdName: crontabs.stable.example.com
  conversions:
  - fromVersion: alpha
    toVersion: beta
  - fromVersion: beta
    toVersion: gamma
  - fromVersion: gamma
    toVersion: delta
- name: down_conversions
  crdName: crontabs.stable.example.com
  conversions:
  - fromVersion: delta
    toVersion: gamma
  - fromVersion: gamma
    toVersion: beta
  - fromVersion: beta
    toVersion: alpha
EOF

else
  exit 0
fi