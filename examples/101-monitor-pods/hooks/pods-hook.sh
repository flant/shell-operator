#!/usr/bin/env bash

if [[ $1 == "--config" ]] ; then
  cat <<EOF
{"onKubernetesEvent":[
  {"kind": "Pod",
  "event":["add"]
  }
]}
EOF
else
  podName=$(jq -r '.[0].resourceName' $BINDING_CONTEXT_PATH)
  echo "Pod '${podName}' added"
fi
