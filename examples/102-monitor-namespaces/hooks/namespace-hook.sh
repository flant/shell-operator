#!/usr/bin/env bash

if [[ $1 == "--config" ]] ; then
  cat <<EOF
{"onKubernetesEvent":[
  {"name":"OnCreateDeleteNamespace",
  "kind": "namespace",
  "event":["add", "delete"]
  },
  {"name":"OnModifiedNamespace",
  "kind": "namespace",
  "event":["update"],
  "jqFilter": ".metadata.labels"
  }
]}
EOF
else
  bindingName=$(jq -r '.[0].binding' $BINDING_CONTEXT_PATH)
  resourceEvent=$(jq -r '.[0].resourceEvent' $BINDING_CONTEXT_PATH)
  resourceName=$(jq -r '.[0].resourceName' $BINDING_CONTEXT_PATH)

  if [[ $bindingName == "OnModifiedNamespace" ]] ; then
    echo "Namespace $resourceName labels were modified"
  else
    if [[ $resourceEvent == "add" ]] ; then
      echo "Namespace $resourceName was created"
    else
      echo "Namespace $resourceName was deleted"
    fi
  fi
fi
