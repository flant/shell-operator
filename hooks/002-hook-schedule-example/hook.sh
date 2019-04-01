#!/usr/bin/env bash

if [[ $1 == "--config" ]] ; then
  echo '{}'
  #echo '{"onStartup": 10, "schedule":[{"name":"every 10sec", "crontab":"*/10 * * * * *"}]}'
  exit 0
fi

echo "002-hook-schedule-example onStartup run"
#echo "Environment Variables"
#export
echo "Binding context:"
cat "${BINDING_CONTEXT_PATH}"
echo "002-hook-schedule-example Stop"

