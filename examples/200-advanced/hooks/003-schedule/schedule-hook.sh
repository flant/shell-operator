#!/usr/bin/env bash

if [[ $1 == "--config" ]] ; then
  cat <<EOF
{"schedule": [
{ "name":"every 10 min",
  "crontab":"* */10 * * * *"
}
]}
EOF
else
  echo "Message from Schedule hook!"
fi
