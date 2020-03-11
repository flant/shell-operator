#!/usr/bin/env bash

if [[ $1 == "--config" ]] ; then
  cat <<EOF
{
  "configVersion": "v1",
  "onStartup": 5,
  "kubernetes": [
    {
      "name":"OnCreateDeleteNamespace",
      "apiVersion": "v1",
      "kind": "namespace",
      "executeHookOnEvent":["Added", "Deleted"]
    },
    {
      "name": "OnModifiedNamespace",
      "apiVersion": "v1",
      "kind": "namespace",
      "executeHookOnEvent": ["Modified"],
      "jqFilter": ".metadata.labels"
  }
]}
EOF
else
  bindingName=$(jq -r '.[0].binding' $BINDING_CONTEXT_PATH)

  if [[ $bindingName == "onStartup" ]] ; then
    echo "namespace-hook is triggered on startup."
    exit 0
  fi

  # ignore Synchronization for simplicity
  type=$(jq -r '.[0].type' $BINDING_CONTEXT_PATH)
  if [[ $type == "Synchronization" ]] ; then
    echo Got Synchronization event
    exit 0
  fi

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
