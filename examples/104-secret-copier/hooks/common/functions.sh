function common::run_hook() {
  if [[ $1 == "--config" ]] ; then
    hook::config
  else
    hook::trigger
  fi
}

function kubectl::replace_or_create() {
  object=$(cat)

  if ! kubectl get -f - <<< "$object" >/dev/null 2>/dev/null; then
    kubectl create -f - <<< "$object" >/dev/null
  else
    kubectl replace --force -f - <<< "$object" >/dev/null
  fi
}
