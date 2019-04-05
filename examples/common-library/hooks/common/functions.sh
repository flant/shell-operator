common::run_hook() {
  if [[ $1 == "--config" ]] ; then
    hook::config
  else
    hook::trigger
  fi
}