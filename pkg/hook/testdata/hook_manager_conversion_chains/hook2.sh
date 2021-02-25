#!/usr/bin/env bash

if [[ $1 == "--config" ]] ; then
    cat <<EOF
configVersion: v1
onStartup: 10
kubernetesCustomResourceConversion:
- name: down_conversions
  crdName: crontabs.unstable.example.com
  conversions:
  - fromVersion: one
    toVersion: two
  - fromVersion: two
    toVersion: three
EOF

else
  exit 0
fi
