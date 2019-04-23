#!/usr/bin/env bash

if [[ $1 == "--config" ]] ; then
  cat <<EOF
{"schedule": [
{ "name":"every 15 min",
  "crontab":"0 */15 * * * *"
}
]}
EOF
else
  echo "Message from Schedule hook!"
fi
