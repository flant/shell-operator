#!/usr/bin/env bash

# This hook is just to have entry for another CRD.

if [[ $1 == "--config" ]] ; then
    cat <<EOF
configVersion: v1
onStartup: 10
kubernetesCustomResourceConversion:
- name: down_conversions
  crdName: crontabs.unstable.example.com
  conversions:
  - fromVersion: alpha.example.com/v1beta1
    toVersion: alpha.example.io/v1beta1
  - fromVersion: alpha.example.io/v1beta1
    toVersion: v1beta2
  - fromVersion: v1beta2
    toVersion: v1
EOF

else
  exit 0
fi
