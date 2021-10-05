#!/bin/bash

# This script can be used to run integration tests locally.
# Set CLUSTER_NAME to use existing cluster for tests.
# i.e. $ CLUSTER_NAME=kube-19 ./test/integration/run.sh

# Start cluster using kind. Delete it on interrupt or on exit.
DEFAULT_CLUSTER_NAME="e2e-test"
KIND_NODE_IMAGE="kindest/node:v1.19.7"

cleanup() {
  kind delete cluster --name $CLUSTER_NAME
}

if [[ -z $CLUSTER_NAME ]] ; then
  echo "Start new cluster"
  CLUSTER_NAME="${DEFAULT_CLUSTER_NAME}"

  trap '(exit 130)' INT
  trap '(exit 143)' TERM
  trap 'rc=$?; cleanup; exit $rc' EXIT

  kind create cluster \
     --name $CLUSTER_NAME \
     --image $KIND_NODE_IMAGE
else
  echo "Use existing cluster '${CLUSTER_NAME}'"
fi


./ginkgo \
  --tags 'integration test'  \
  --vet off \
  --race \
  -p \
  -r test/integration
