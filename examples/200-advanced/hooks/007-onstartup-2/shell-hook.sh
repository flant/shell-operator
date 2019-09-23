#!/usr/bin/env bash

if [[ $1 == "--config" ]] ; then
  echo '{"configVersion":"v1", "onStartup": 2}'
else
  echo "007-onstartup-2 hook is triggered"
fi
