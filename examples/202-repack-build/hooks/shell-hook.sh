#!/usr/bin/env bash

if [[ $1 == "--config" ]] ; then
  echo '{"configVersion":"v1", "onStartup": 1}'
else
  echo "OnStartup shell hook"
fi
