#!/bin/bash

function hook::run() {
  if [[ "${1:-}" == "--config" ]] ; then
    __config__
    exit 0
  fi

  CONTEXT_LENGTH=$(context::global::jq -r 'length')
  for i in `seq 0 $((CONTEXT_LENGTH - 1))`; do
    export BINDING_CONTEXT_CURRENT_INDEX="${i}"
    export BINDING_CONTEXT_CURRENT_BINDING=$(context::jq -r '.binding // "unknown"')

    HANDLERS=$(hook::_get_possible_handler_names)
    HANDLERS="${HANDLERS} __main__"

    hook::_run_first_available_handler "${HANDLERS}"
  done
}

function hook::_get_possible_handler_names() {
  if [[ "$BINDING_CONTEXT_CURRENT_BINDING" == "onStartup" ]]; then
    echo __on_startup
  elif BINDING_CONTEXT_CURRENT_TYPE=$(context::jq -er '.type'); then
    case "${BINDING_CONTEXT_CURRENT_TYPE}" in
    "Synchronization")
      echo __on_kubernetes::${BINDING_CONTEXT_CURRENT_BINDING}::synchronization
      echo __on_kubernetes::${BINDING_CONTEXT_CURRENT_BINDING}
    ;;
    "Event")
      case "$(context::jq -r '.watchEvent')" in
      "Added")
        echo __on_kubernetes::${BINDING_CONTEXT_CURRENT_BINDING}::added
        echo __on_kubernetes::${BINDING_CONTEXT_CURRENT_BINDING}::added_or_modified
        echo __on_kubernetes::${BINDING_CONTEXT_CURRENT_BINDING}
      ;;
      "Modified")
        echo __on_kubernetes::${BINDING_CONTEXT_CURRENT_BINDING}::modified
        echo __on_kubernetes::${BINDING_CONTEXT_CURRENT_BINDING}::added_or_modified
        echo __on_kubernetes::${BINDING_CONTEXT_CURRENT_BINDING}
      ;;
      "Deleted")
        echo __on_kubernetes::${BINDING_CONTEXT_CURRENT_BINDING}::deleted
        echo __on_kubernetes::${BINDING_CONTEXT_CURRENT_BINDING}
      ;;
      esac
    ;;
    "Group")
      echo __on_group::${BINDING_CONTEXT_CURRENT_BINDING}
    ;;
    "Schedule")
      echo __on_schedule::${BINDING_CONTEXT_CURRENT_BINDING}
    ;;
    "Validating")
      echo __on_validating::${BINDING_CONTEXT_CURRENT_BINDING}
    ;;
    "Mutating")
      echo __on_mutating::${BINDING_CONTEXT_CURRENT_BINDING}
    ;;
    "Conversion")
      echo __on_conversion::${BINDING_CONTEXT_CURRENT_BINDING}::$(context::jq -er '[.fromVersion,.toVersion]| map(sub("/";".")) | join("::")')
      echo __on_conversion::${BINDING_CONTEXT_CURRENT_BINDING}
    ;;
    esac
  fi
}

function hook::_run_first_available_handler() {
  HANDLERS="$1"

  for handler in ${HANDLERS}; do
    if type $handler >/dev/null 2>&1; then
      ($handler) # brackets are to run handler as a subprocess
      return $?
    fi
  done

  >&2 printf "ERROR: Can't find any handler from the list: %s\n." "$(echo ${HANDLERS} | sed -E 's/[[:space:]]+/, /g')"
  return 1
}
