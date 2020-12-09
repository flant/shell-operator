#!/usr/bin/env bash

source /shell_lib.sh

function __config__(){
    cat <<EOF
configVersion: v1
onStartup: 10

EOF
}

function __main__() {
  echo "VALIDATING ON STARTUP"

  ls -la /
  ls -la /validating-certs

  # example of validating response
  cat <<EOF >> $VALIDATING_RESPONSE_PATH
  {"allow":false, "result":{"message":"No money no honey", "code": 403}}
EOF
}



hook::run $@