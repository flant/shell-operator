#!/usr/bin/env bash

if [[ $1 == "--config" ]] ; then
  cat <<EOF
{
  "configVersion":"v1",
  "schedule": [
    {
      "name": "every 1 min",
      "crontab": "*/1 * * * *"
    },
    {
      "name": "every 5 min",
      "crontab": "*/5 * * * *"
    }
  ]
}
EOF
else
  binding=$(cat $BINDING_CONTEXT_PATH)
  echo "Message from 'schedule' hook with 5 fields crontab: $binding"
fi
