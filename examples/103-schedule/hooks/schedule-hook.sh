#!/usr/bin/env bash

if [[ $1 == "--config" ]] ; then
  cat <<EOF
{
  "configVersion":"v1",
  "schedule": [
    {
      "name":"every 10 sec",
      "crontab":"*/10 * * * * *"
    },
    {
      "name":"every 5 sec",
      "crontab":"*/5 * * * * *"
    },
    {
      "name":"every 10 min",
      "crontab":"0 */10 * * * *"
    }
  ]
}
EOF
else
  binding=$(cat $BINDING_CONTEXT_PATH)
  echo "Message from Schedule hook: $binding"
fi
