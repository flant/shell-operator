# HOOKs

Hook is a script with the added events configuration code.

Hook script can be written on any language you prefer and it will be executed by a Shell-operator in a Kubernetes cluster on events you configured. A corresponding runtime environment (ex. bash) must be added to the image to execute a hook.

Every hook must have an argument `--config`. Getting `--config` as an argument, hook have to return to `stdout` its [bindings configuration](#bindings) as a JSON structure.

When one or more events with which the hook is binded are triggered, Shell-operator executes the hook without arguments but with the following enviroment variables set:
- BINDING_CONTEXT_PATH - contains a path to a JSON file with an array of objects on which the events were triggered.
- WORKING_DIR - contains a path to a hooks directory. It can be helpful when you use shared libraries in your hooks.

### hook configuration

Shell-operator on startup run every hook with `--config` argument.



### bingings

* onStartup
* schedule
* onKubernetesEvent

#### onStartup

```
{
  "onStartup": ORDER
}
```



#### schedule



#### onKubernetes
