#!/usr/bin/env bash

source /hooks/common/functions.sh

hook::config() {
  echo '{"configVersion": "v1", "onStartup": 10 }'
}

hook::trigger() {
  echo "hook::trigger function is called!"
}

common::run_hook "$@"
