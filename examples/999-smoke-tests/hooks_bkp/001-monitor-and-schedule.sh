#!/bin/bash

if [[ $1 == "--config" ]] ; then
  cat <<EOF
configVersion: v1
kubernetes:
- name: monitor-pods
  apiVersion: v1
  kind: Pod
  executeHookOnEvent: ["Added", "Modified"]
schedule:
- name: ten-seconds-snapshot
  crontab: "*/10 * * * * *"
  includeSnapshotsFrom: ["monitor-pods"]
EOF
else
  binding_name=$(jq -r '.[0].binding' $BINDING_CONTEXT_PATH)

  if [[ $binding_name == "monitor-pods" ]] ; then
    pod_name=$(jq -r '.[0].objects[0].object.metadata.name' $BINDING_CONTEXT_PATH)
    echo "Pod '${pod_name}' is Added or Modified."
  elif [[ $binding_name == "ten-seconds-snapshot" ]] ; then
    pod_count=$(jq -r '.[0].snapshots."monitor-pods" | length' $BINDING_CONTEXT_PATH)
    echo "Schedule event: there are ${pod_count} pods in the cluster."
  fi
fi 