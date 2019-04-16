#!/usr/bin/env bash

source $WORKING_DIR/common/functions.sh

hook::config() {
  echo '{"onStartup": 10 }'
}

hook::trigger() {
  echo "001-onstartup-10 hook is triggered"
}

common::run_hook "$@"
