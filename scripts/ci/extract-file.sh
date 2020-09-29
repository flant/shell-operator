#!/usr/bin/env bash

function usage() {
  cat <<EOF
Usage: $0 image:tag /path/to/file/in/image /path/to/file/on/host

Example:
  $0 /app/jq /app/out
EOF
}

IMAGE=$1
if [[ $IMAGE == "" ]] ; then
  usage
  exit 1
fi

FILE=$2
if [[ $FILE == "" ]] ; then
  usage
  exit 1
fi

OUT=$3
if [[ $OUT == "" ]] ; then
  usage
  exit 1
fi

container_id=$(docker create $IMAGE)  # returns container ID
echo $container_id
docker cp $container_id:$FILE $OUT
docker rm $container_id
