# HOOKs

Hook is a script with the added events configuration code.

Hook script can be written on any language you prefer and it will be executed by a Shell-operator in a Kubernetes cluster on events you configured. A corresponding runtime environment (ex. bash) must be added to the image to execute a hook.

Every hook must have an argument `--config`. Getting `--config` as an argument, hook have to return to `stdout` it's [bindings configuration](#bindings) as a JSON structure.

When one or more events with which the hook is binded are triggered, Shell-operator executes the hook without arguments but with the following enviroment variables set:
- BINDING_CONTEXT_PATH - contains a path to a JSON file with an array of objects on which the events were triggered.
- WORKING_DIR - contains a path to a hooks directory. It can be helpful when you use shared libraries in your hooks.

## How hooks run

Shell-operator searches and runs hooks going through the following steps:
- Recursively searches hooks in a working directory which is `hooks` by default and can be specified by a WORKING_DIR environment variable or a `--working-dir` argument (overrides WORKING_DIR):
    - directories starting from dot symbol are excluded
    - directories and files with it alphabetically sorted
    - every **executable** file not starting from dot symbol counts as hook
- Executes hooks in the order found to get hooks configuration:
    - each hook is executed with the `--config` argument in the hook directory in which it is located
    - the WORKING_DIR enviroment variable is set
    - hook returns to `stdout` a JSON array of it's [bindings configuration](#bindings)
- Executes hooks depending on the schedule and Kubernetes events:
    - if there is more than one hook for a triggered event, hooks executed squentially in the same order they have been found.

Each hook is executed one by one (without concurrency) and each hook is executed until it finishes. On that note you may have a question - "What happens if one of the hooks fails"?

### What happens if hooks fails?

If
- writes the corresponding message in the log
- waits for 3 seconds
- tries to execute the hook again... and again... and again

If you need to skeep, you can set `allowFailure: yes`.


### hook configuration

Hook can have more than one bindings.

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


Here is an example of a BINDING_CONTEXT_PATH content:
```
#TODO
```
