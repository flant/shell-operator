#!/usr/bin/env bash

if [[ $1 == "--config" ]] ; then
  cat <<EOF
configVersion: v1
kubernetes:
- name: monitor-configmaps
  apiVersion: v1
  kind: ConfigMap
  executeHookOnEvent:
  - Added
  - Modified
  - Deleted
  namespace:
    nameSelector:
      matchNames:
      - example-monitor-configmap
  # Run the hook on "Modified" only if the data is changed,
  # changes in labels, annotations, etc. are ignored.
  jqFilter: ".data"
EOF
else
  # Several events can be combined into one hook run, so handle all binding contexts.
  contextsCount=$(jq length "$BINDING_CONTEXT_PATH")
  for (( i = 0; i < contextsCount; i++ )) ; do
    type=$(jq -r ".[$i].type" "$BINDING_CONTEXT_PATH")

    if [[ $type == "Synchronization" ]] ; then
      # On startup the hook receives all existing ConfigMaps.
      for name in $(jq -r ".[$i].objects[].object.metadata.name" "$BINDING_CONTEXT_PATH") ; do
        echo "ConfigMap '${name}' exists"
      done
    fi

    if [[ $type == "Event" ]] ; then
      name=$(jq -r ".[$i].object.metadata.name" "$BINDING_CONTEXT_PATH")
      # filterResult is the result of jqFilter, i.e. the data of the ConfigMap.
      data=$(jq -c ".[$i].filterResult" "$BINDING_CONTEXT_PATH")
      case $(jq -r ".[$i].watchEvent" "$BINDING_CONTEXT_PATH") in
        Added)
          echo "ConfigMap '${name}' added with data ${data}"
          ;;
        Modified)
          echo "ConfigMap '${name}' data changed to ${data}"
          ;;
        Deleted)
          echo "ConfigMap '${name}' deleted"
          ;;
      esac
    fi
  done
fi
