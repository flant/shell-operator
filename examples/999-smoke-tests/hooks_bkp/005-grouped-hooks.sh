#!/bin/bash

if [[ $1 == "--config" ]] ; then
  cat <<EOF
configVersion: v1
kubernetes:
- name: monitor-services
  apiVersion: v1
  group: services-and-deployments
  kind: Service
  executeHookOnEvent: ["Added"]
- name: monitor-deployments
  apiVersion: apps/v1
  group: services-and-deployments
  kind: Deployment
  executeHookOnEvent: ["Added"]
EOF
else
  binding_name=$(jq -r '.[0].binding' $BINDING_CONTEXT_PATH)
  object_kind=$(jq -r '.[0].object.kind' $BINDING_CONTEXT_PATH)
  object_name=$(jq -r '.[0].object.metadata.name' $BINDING_CONTEXT_PATH)
  
  echo "Grouped hook event: binding='${binding_name}', kind='${object_kind}', name='${object_name}'"
fi 