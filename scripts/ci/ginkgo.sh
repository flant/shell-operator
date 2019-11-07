#!/bin/bash -e

workspace=$1
mkdir -p $workspace/bin

go build github.com/onsi/ginkgo/ginkgo
mv ginkgo $workspace/bin
