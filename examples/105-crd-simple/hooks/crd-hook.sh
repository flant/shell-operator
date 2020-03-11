#!/usr/bin/env bash

if [[ $1 == "--config" ]] ; then
  cat <<EOF
{
  "configVersion":"v1",
  "kubernetes":[{
    "apiVersion": "stable.example.com/v1",
    "kind": "Crontab",
    "executeHookOnEvent":["Added"]
  }]
}
EOF
else
  type=$(jq -r '.[0].type' ${BINDING_CONTEXT_PATH})
  if [[ $type == "Synchronization" ]] ; then
    : handle existing objects
    : jq '.[0].objects | ... '
  fi

  if [[ $type == "Event" ]] ; then
    name=$(jq -r '.[0].object.metadata.name' ${BINDING_CONTEXT_PATH})
    kind=$(jq -r '.[0].object.kind' ${BINDING_CONTEXT_PATH})
    echo "${kind}/${name} object is added"
  fi
fi
