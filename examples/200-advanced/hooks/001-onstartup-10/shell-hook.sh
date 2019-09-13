#!/usr/bin/env bash

source /hooks/common/functions.sh

hook::config() {
  echo '{"configVersion":"v1", "onStartup": 10 }'
}

hook::trigger() {
  echo "001-onstartup-10 hook is triggered"
}

common::run_hook "$@"
