#!/usr/bin/env bash

if [[ $1 == "--config" ]] ; then
  echo '{"onStartup": 10}'
  exit 0
fi

echo "001-hook-first onStartup run"

source config