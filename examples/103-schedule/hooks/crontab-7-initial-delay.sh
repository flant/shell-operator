#!/usr/bin/env bash

if [[ $1 == "--config" ]] ; then
  cat <<EOF
{
  "configVersion":"v1",
  "schedule": [
    {
      "name": "every 10 sec with initial delay 10 sec",
      "crontab": "*/10 * * * * *",
      "firstRunDelay": "10s"
    },
  ]
}
EOF
else
  binding=$(cat $BINDING_CONTEXT_PATH)
  echo "Message from 'schedule' hook with 7 fields crontab: $binding"
fi
