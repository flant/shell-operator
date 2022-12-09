#!/usr/bin/env bash

source /shell_lib.sh

function __config__(){
    cat <<EOF
configVersion: v1
kubernetesMutating:
- name: change.crontab.resources
  namespace:
    labelSelector:
      matchLabels:
        # helm adds a 'name' label to a namespace it creates
        name: example-206
  rules:
  - apiGroups:   ["stable.example.com"]
    apiVersions: ["v1"]
    operations:  ["CREATE", "UPDATE"]
    resources:   ["crontabs"]
    scope:       "Namespaced"
EOF
}

function __main__() {
  context::jq -r '.review.request'
#  image=$(context::jq -r '.review.request.object.spec.image')
#  echo "Got image: $image"

#  if [[ $image == repo.example.com* ]] ; then
  PATCH=$( echo '[{"op": "add", "path": "/spec/replicas", "value": 333 }]' | base64 )
    cat <<EOF > $VALIDATING_RESPONSE_PATH
{"allowed":true, "patch": "$PATCH"}
EOF
#  else
#    cat <<EOF > $VALIDATING_RESPONSE_PATH
#{"allowed":false, "message":"Only images from repo.example.com are allowed"}
#EOF
#  fi
}

hook::run $@
