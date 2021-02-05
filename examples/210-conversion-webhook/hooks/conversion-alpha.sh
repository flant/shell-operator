#!/usr/bin/env bash

source $FRAMEWORK_DIR/shell_lib.sh

function __config__() {
  cat <<EOF
configVersion: v1
kubernetesCustomResourceConversion:
  - name: alpha1_to_alpha2
    crdName: crontabs.stable.example.com
    conversions:
    - fromVersion: stable.example.com/v1alpha1
      toVersion: stable.example.com/v1alpha2
EOF
}

function __on_conversion::alpha1_to_alpha2() {
  if converted=$(context::jq -r '.review.request.objects//[] | map(
     if .apiVersion == "stable.example.com/v1alpha1" then
       # Bump a version
       .apiVersion = "stable.example.com/v1alpha2" |
       # Transform spec field.
       .spec.cron = (.spec.cron | join(" "))
     else . end
  )')
  then
    cat <<EOF >$CONVERSION_RESPONSE_PATH
{"convertedObjects": $converted}
EOF
  else
    cat <<EOF >$CONVERSION_RESPONSE_PATH
{"failedMessage":"Conversion of crontabs.stable.example.com is failed"}
EOF
  fi
}

hook::run $@
