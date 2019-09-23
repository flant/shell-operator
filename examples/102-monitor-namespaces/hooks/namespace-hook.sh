#!/usr/bin/env bash

if [[ $1 == "--config" ]] ; then
  cat <<EOF
{
  "configVersion":"v1",
  "kubernetes":[
  {
    "name":"OnCreateDeleteNamespace",
    "kind": "namespace",
    "watchEvent":["Added", "Deleted"]
  },
  {
    "name":"OnModifiedNamespace",
    "kind": "namespace",
    "watchEvent":["Modified"],
    "jqFilter": ".metadata.labels"
  }
]}
EOF
else
  # ignore Synchronization for simplicity
  type=$(jq -r '.[0].type' $BINDING_CONTEXT_PATH)
  if [[ $type == "Synchronization" ]] ; then
    echo Got Synchronization event
    exit 0
  fi

  bindingName=$(jq -r '.[0].binding' $BINDING_CONTEXT_PATH)
  resourceEvent=$(jq -r '.[0].watchEvent' $BINDING_CONTEXT_PATH)
  resourceName=$(jq -r '.[0].object.metadata.name' $BINDING_CONTEXT_PATH)

  if [[ $bindingName == "OnModifiedNamespace" ]] ; then
    echo "Namespace $resourceName labels were modified"
  else
    if [[ $resourceEvent == "Added" ]] ; then
      echo "Namespace $resourceName was created"
    else
      echo "Namespace $resourceName was deleted"
    fi
  fi
fi
