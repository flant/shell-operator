#!/bin/bash

function context::global::jq() {
  jq "$@" ${BINDING_CONTEXT_PATH}
}

# $1 — jq filter for current context
function context::jq() {
  context::global::jq '.['"${BINDING_CONTEXT_CURRENT_INDEX}"']' | jq "$@"
}

function context::get() {
  local required=no

  while true ; do
    case ${1:-} in
      --required)
        required=yes
        shift
        ;;
      *)
        break
        ;;
    esac
  done

  if [[ "$required" == "yes" ]] && ! context::has "${1:-}"; then
      >&2 echo "Error: Value $1 required, but doesn't exist"
      return 1
  fi

  jqPath="$(context::_convert_user_path_to_jq_path "${1:-}")"
  context::jq -r "${jqPath}"
}

function context::has() {
  local path=$(context::_dirname "${1:-}")
  local key=$(context::_basename "${1:-}")

  quotes='"'
  if [[ "$key" =~ ^[0-9]+$ ]]; then
    quotes=''
  fi

  jqPath="$(context::_convert_user_path_to_jq_path "${path}")"
  context::jq -e "${jqPath} | has(${quotes}${key}${quotes})" >/dev/null
}

function context::is_true() {
  jqPath="$(context::_convert_user_path_to_jq_path "${1:-}")"
  context::jq -e "${jqPath} == true" >/dev/null
}

function context::is_false() {
  jqPath="$(context::_convert_user_path_to_jq_path "${1:-}")"
  context::jq -e "${jqPath} == false" >/dev/null
}

function context::is_null() {
  jqPath="$(context::_convert_user_path_to_jq_path "${1:-}")"
  context::jq -e "${jqPath} == null" >/dev/null
}

function context::_convert_user_path_to_jq_path() {
  # change single-quote to double-quote (' -> ")
  # loop1 — hide dots in keys, i.e. aaa."bb.bb".ccc -> aaa."bb##DOT##bb".cc
  # loop2 — quote keys with symbol "-", i.e. 'myModule.internal.address-pool' -> 'myModule.internal."address-pool"'
  # loop3 — convert array addresation from myArray.0 to myArray[0]
  # loop4 — return original dots from ##DOT##, i.e. aaa."bb##DOT##bb".cc -> aa."bb.bb".cc

  jqPath=".$(sed -r \
    -e s/\'/\"/g \
    -e ':loop1' -e 's/"([^".]+)\.([^"]+)"/"\1##DOT##\2"/g' -e 't loop1' \
    -e ':loop2' -e 's/(^|\.)([^."]*-[^."]*)(\.|$)/\1"\2"\3/g' -e 't loop2' \
    -e ':loop3' -e 's/(^|\.)([0-9]+)(\.|$)/[\2]\3/g' -e 't loop3' \
    -e ':loop4' -e 's/(^|\.)"([^"]+)##DOT##([^"]+)"(\.|$)/\1"\2.\3"\4/g' -e 't loop4' \
    <<< "${1:-}"
  )"
  echo "${jqPath}"
}

function context::_dirname() {
  # loop1 — hide dots in keys, i.e. aaa."bb.bb".ccc -> aaa."bb##DOT##bb".cc
  splittable_path="$(sed -r -e s/\'/\"/g -e ':loop1' -e 's/"([^".]+)\.([^"]+)"/"\1##DOT##\2"/g' -e 't loop1' <<< ${1:-})"

  # loop2 — return original dots from ##DOT##, i.e. aaa."bb##DOT##bb".cc -> aa."bb.bb".cc
  rev <<< "${splittable_path}" | cut -d. -f2- | rev | sed -r -e ':loop2' -e 's/(^|\.)"([^"]+)##DOT##([^"]+)"(\.|$)/\1"\2.\3"\4/g' -e 't loop2'
}

function context::_basename() {
  # loop1 — hide dots in keys, i.e. aaa."bb.bb".ccc -> aaa."bb##DOT##bb".cc
  splittable_path="$(sed -r -e s/\'/\"/g -e ':loop1' -e 's/"([^".]+)\.([^"]+)"/"\1##DOT##\2"/g' -e 't loop1' <<< ${1:-})"

  # loop2 — return original dots from ##DOT##, i.e. "bb##DOT##bb" -> bb.bb
  rev <<< "${splittable_path}" | cut -d. -f1 | rev | sed -r -e ':loop2' -e 's/^"([^"]+)##DOT##([^"]+)"$/\1.\2/g' -e 't loop2'
}
