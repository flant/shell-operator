#!/usr/bin/env bash

if [[ $1 == "--config" ]] ; then
  echo '{"onStartup": 10, "schedule":[{"name":"every 10sec", "crontab":"*/10 * * * * *"}]}'
  exit 0
fi

echo "002-hook-schedule-example onStartup run"
echo "Binding context:"
cat "${BINDING_CONTEXT_PATH}"
echo
echo "002-hook-schedule-example Stop"
