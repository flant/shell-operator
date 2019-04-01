#!/usr/bin/env bash

if [[ $1 == "--config" ]] ; then
  cat <<EOF
{
  "onStartup": 1,
  "onKubernetesEvent": [
    {
      "name": "myLabelValue pods",
      "kind": "pod",
      "event": ["add", "delete"],
      "selector": {
        "matchLabels": {
          "myLabel": "myLabelValue"
        }
      },
      "namespaceSelector": {
        "any": true
      },
      "jqFilter": ".metadata.labels",
      "allowFailure": true
    }
  ]
}
EOF
  exit 0
fi

echo "003-hook-kube-example onStartup run"
echo "Binding context:"
cat "${BINDING_CONTEXT_PATH}"
echo "003-hook-kube-example Stop"
