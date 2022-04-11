#!/usr/bin/env bash

if [[ $1 == "--config" ]] ; then
cat <<EOF
configVersion: v1
onStartup: 10
kubernetes:
- name: monitor-pods
  kind: Pod
  executeHookOnSynchronization: false
schedule:
- crontab: "* * * * *"
EOF
else
echo "hook_one"
fi
