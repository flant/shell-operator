#!/usr/bin/env bash

if [[ $1 == "--config" ]] ; then
  cat <<EOF
configVersion: v1
kubernetes:
- apiVersion: v1
  kind: Pod
  executeHookOnEvent:
  - Added
EOF
else
  jq -c '.[]' $BINDING_CONTEXT_PATH | while read -r event; do
  podName=$(echo "$event" | jq -r '.object.metadata.name')
  echo "Pod '${podName}' was in the batch"
  done
fi
