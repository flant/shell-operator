#!/usr/bin/env bash

if [[ $1 == "--config" ]] ; then
  cat <<EOF
configVersion: v1
settings:
  executionMinInterval: 5s
  executionBurst: 2
kubernetes:
- name: crontabs
  apiVersion: stable.example.com/v1
  kind: ct
EOF
exit 0
fi


echo "settings-rate-limit on kube"

CONTEXT_LENGTH=$(jq -r '.|length' ${BINDING_CONTEXT_PATH})
echo "RateLimitedHook binding contexts: ${CONTEXT_LENGTH}"
