#!/usr/bin/env bash

ARRAY_COUNT=`jq -r '. | length-1' $BINDING_CONTEXT_PATH`

if [[ $1 == "--config" ]] ; then
  cat <<EOF
configVersion: v1
kubernetes:
- name: OnCreateDeleteNamespace
  apiVersion: v1
  kind: Namespace
  executeHookOnEvent:
  - Added
  - Deleted
- name: OnModifiedNamespace
  apiVersion: v1
  kind: Namespace
  executeHookOnEvent:
  - Modified
  jqFilter: ".metadata.labels"
EOF
else
  # ignore Synchronization for simplicity
  type=$(jq -r '.[0].type' $BINDING_CONTEXT_PATH)
  if [[ $type == "Synchronization" ]] ; then
    echo Got Synchronization event
    exit 0
  fi

  for IND in `seq 0 $ARRAY_COUNT`
  do
    bindingName=`jq -r ".[$IND].binding" $BINDING_CONTEXT_PATH`
    resourceEvent=`jq -r ".[$IND].watchEvent" $BINDING_CONTEXT_PATH`
    resourceName=`jq -r ".[$IND].object.metadata.name" $BINDING_CONTEXT_PATH`

    if [[ $bindingName == "OnModifiedNamespace" ]] ; then
      echo "Namespace $resourceName labels were modified"
    else
      if [[ $resourceEvent == "Added" ]] ; then
        echo "Namespace $resourceName was created"
      else
        echo "Namespace $resourceName was deleted"
      fi
    fi
  done
fi
