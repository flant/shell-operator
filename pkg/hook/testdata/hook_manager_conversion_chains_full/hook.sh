#!/usr/bin/env bash

if [[ $1 == "--config" ]] ; then
    cat <<EOF
configVersion: v1
onStartup: 10
kubernetesCustomResourceConversion:
- name: up_conversions
  crdName: crontabs.stable.example.com
  conversions:
  - fromVersion: group.io/alpha
    toVersion: beta
  - fromVersion: unstable.example.com/beta
    toVersion: gamma
  - fromVersion: stable.example.com/gamma
    toVersion: next.io/delta
- name: down_conversions
  crdName: crontabs.stable.example.com
  conversions:
  - fromVersion: stable.example.com/delta
    toVersion: stable.example.com/gamma
  - fromVersion: gamma
    toVersion: beta
  - fromVersion: beta
    toVersion: alpha
EOF

else
  exit 0
fi