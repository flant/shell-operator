#!/usr/bin/env bash

if [[ $1 == "--config" ]] ; then
  cat <<EOF
{
  "configVersion":"v1",
  "schedule": [
    {
      "name": "every 15 min",
      "crontab": "*/15 * * * *"
    }
  ]
}
EOF
else
  echo "Message from Schedule hook!"
fi
