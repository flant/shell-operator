#!/usr/bin/env bash

if [[ $1 == "--config" ]] ; then
cat <<EOF
configVersion: v1
onStartup: 20
kubernetes:
- name: monitor-pods
  kind: Pod
  executeHookOnSynchronization: false
EOF
else
echo "hook_one"
fi
