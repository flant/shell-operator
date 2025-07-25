#!/bin/bash

if [[ $1 == "--config" ]] ; then
  cat <<EOF
configVersion: v1
kubernetes:
- name: patch-pod
  apiVersion: v1
  kind: Pod
  jqFilter: '.metadata.annotations'
  executeHookOnEvent: ["Added", "Modified"]
EOF
else
  type=$(jq -r '.[0].type' $BINDING_CONTEXT_PATH)
  if [[ $type == "Event" ]] ; then
    # Note: we use .object here because we need the full object metadata for the patch.
    # .filterResult would also work in this case because of 'select', but using .object is safer.
    pod_name=$(jq -r '.[0].object.metadata.name' $BINDING_CONTEXT_PATH)
    namespace=$(jq -r '.[0].object.metadata.namespace' $BINDING_CONTEXT_PATH)
    echo "Pod '${pod_name}' in namespace '${namespace}"
  fi
fi 