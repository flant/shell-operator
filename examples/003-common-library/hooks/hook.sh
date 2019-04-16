#!/usr/bin/env bash

source $WORKING_DIR/common/functions.sh

hook::config() {
  echo '{"onStartup": 10 }'
}

hook::trigger() {
  echo "hook::trigger function is called!"
}

common::run_hook "$@"
