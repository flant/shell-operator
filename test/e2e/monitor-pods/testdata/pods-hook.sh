#!/usr/bin/env bash

if [[ $1 == "--config" ]] ; then
  cat <<EOF
{
  "configVersion":"v1",
  "kubernetes":[{
    "apiVersion": "v1",
    "kind": "Pod",
    "watchEvent":["Added"]
  }]
}
EOF
else
  type=$(jq -r '.[0].type' $BINDING_CONTEXT_PATH)
  if [[ $type == "Event" ]] ; then
    podName=$(jq -r '.[0].object.metadata.name' $BINDING_CONTEXT_PATH)
    echo "Pod '${podName}' added"
  else
    echo "Synchronization run"
  fi
fi
