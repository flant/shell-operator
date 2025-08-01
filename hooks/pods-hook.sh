#!/usr/bin/env bash

if [[ $1 == "--config" ]] ; then
  cat <<EOF
configVersion: v1
kubernetes:
- apiVersion: v1
  kind: Pod
  executeHookOnEvent:
  - Added
  - Modified
  - Deleted
EOF
else
  jq -c '.[]' $BINDING_CONTEXT_PATH | while read -r event; do
    type=$(echo "$event" | jq -r '.type')
    if [[ $type == "Event" ]] ; then
      podName=$(echo "$event" | jq -r '.object.metadata.name')
      echo "Pod '${podName}'"
    fi
  done
fi
