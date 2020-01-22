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

    HANDLER="__undefined"
    HANDLER_SPARE="__undefined"
    HANDLER_SPARE_SPARE="__undefined"
    case "${BINDING_CONTEXT_CURRENT_BINDING}" in
    "onStartup")
      HANDLER="__on_startup"
    ;;
    *)
      # if current context has .type field
      if BINDING_CONTEXT_CURRENT_TYPE=$(context::jq -er '.type'); then
        HANDLER_SPARE_SPARE="__on_kubernetes::${BINDING_CONTEXT_CURRENT_BINDING}"
        HANDLER="__on_kubernetes"
        case "${BINDING_CONTEXT_CURRENT_TYPE}" in
        "Synchronization")
          HANDLER="${HANDLER}::${BINDING_CONTEXT_CURRENT_BINDING}::synchronization"
        ;;
        "Event")
          case "$(context::jq -r '.watchEvent')" in
          "Added")
            HANDLER_SPARE="${HANDLER}::${BINDING_CONTEXT_CURRENT_BINDING}::added_or_modified"
            HANDLER="${HANDLER}::${BINDING_CONTEXT_CURRENT_BINDING}::added"
          ;;
          "Modified")
            HANDLER_SPARE="${HANDLER}::${BINDING_CONTEXT_CURRENT_BINDING}::added_or_modified"
            HANDLER="${HANDLER}::${BINDING_CONTEXT_CURRENT_BINDING}::modified"
          ;;
          "Deleted")
            HANDLER="${HANDLER}::${BINDING_CONTEXT_CURRENT_BINDING}::deleted"
          ;;
          esac
        ;;
        esac
      else
        HANDLER="__on_schedule::${BINDING_CONTEXT_CURRENT_BINDING}"
      fi
    ;;
    esac

    if type $HANDLER >/dev/null 2>&1; then
      $HANDLER
    elif type $HANDLER_SPARE >/dev/null 2>&1; then
      $HANDLER_SPARE
    elif type $HANDLER_SPARE_SPARE >/dev/null 2>&1; then
      $HANDLER_SPARE_SPARE
    elif type __main__ >/dev/null 2>&1; then
      __main__
    else
      >&2 echo -n "ERROR: Can't find handler '${HANDLER}'"
      if [ -n "${HANDLER_SPARE}" ]; then
        >&2 echo -n " or '${HANDLER_SPARE}'"
      fi
      >&2 echo " or '__main__'"
      exit 1
    fi
  done
}
