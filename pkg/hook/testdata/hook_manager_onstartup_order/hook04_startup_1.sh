#!/usr/bin/env bash

if [[ $1 == "--config" ]] ; then
  cat <<EOF
configVersion: v1
onStartup: 1
kubernetes:
- name: pods
  kind: Pod
EOF
fi
