#!/bin/bash

while true ; do
  for i in `seq 1 4` ; do
     (cat <<EOF | kubectl -n example-220 replace --force -f - ; true
apiVersion: "stable.example.com/v1"
kind: CronTab
metadata:
  name: crontab-obj-${i}
  labels:
    foo: bar
spec:
  cron: "= = = = =/10"
  image: some-cron-image-${i}
EOF
)
  done
  sleep 1
done
