#!/usr/bin/env bash

source /shell_lib.sh

function __config__(){
    cat <<EOF
configVersion: v1
kubernetesValidating:
- name: private-repo-policy.example.com
  namespace:
    labelSelector:
      matchLabels:
        # helm adds a 'name' label to a namespace it creates
        name: example-204
  rules:
  - apiGroups:   ["stable.example.com"]
    apiVersions: ["v1"]
    operations:  ["CREATE", "UPDATE"]
    resources:   ["crontabs"]
    scope:       "Namespaced"
EOF
}

function __on_validating::private-repo-policy.example.com() {
  image=$(context::jq -r '.review.request.object.spec.image')
  echo "Got image: $image"

  if [[ $image == repo.example.com* ]] ; then
    cat <<EOF > $VALIDATING_RESPONSE_PATH
{"allowed":true}
EOF
  else
    cat <<EOF > $VALIDATING_RESPONSE_PATH
{"allowed":false, "message":"Only images from repo.example.com are allowed"}
EOF
  fi
}

hook::run $@
