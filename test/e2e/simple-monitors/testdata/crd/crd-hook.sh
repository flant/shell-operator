#!/usr/bin/env bash

if [[ $1 == "--config" ]] ; then
  cat <<EOF
{
  "configVersion":"v1",
  "kubernetes":[{
    "apiVersion": "stable.example.com/v1",
    "kind": "Crontab",
    "watchEvent":["Added"]
  }]
}
EOF
else
  type=$(jq -r '.[0].type' ${BINDING_CONTEXT_PATH})

  if [[ $type == "Synchronization" ]] ; then
    echo "Synchronization run"
    exit 0
  fi

  if [[ $type == "Event" ]] ; then
    objName=$(jq -r '.[0].object.metadata.name' ${BINDING_CONTEXT_PATH})
    objKind=$(jq -r '.[0].object.kind' ${BINDING_CONTEXT_PATH})
    echo "${objKnd}/${objName} is added"
  fi
fi
