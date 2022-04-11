#!/usr/bin/env bash

if [[ $1 == "--config" ]] ; then
cat <<EOF
configVersion: v1
onStartup: 1
schedule:
- crontab: "* * * * *"
EOF
else
echo "hook_two"
fi
