## Schedule example

Example of a hook that runs every 10 minutes.

### run

Build shell-operator image with custom scripts:

```
$ docker build -t "registry.mycompany.com/shell-operator:schedule" .
$ docker push registry.mycompany.com/shell-operator:schedule
```

Edit image in shell-operator-pod.yaml and apply manifests:

```
$ kubectl create ns example-schedule
$ kubectl -n example-schedule apply -f shell-operator-pod.yaml
```

See in logs that schedule-hook.sh was run:

```
$ kubectl -n example-schedule logs -f po/shell-operator
...
INFO     : Running schedule manager entry '*/5 * * * * *' ...
INFO     : Running schedule manager entry '*/10 * * * * *' ...
INFO     : TASK_RUN HookRun@SCHEDULE schedule-hook.sh
INFO     : Running hook 'schedule-hook.sh' binding 'SCHEDULE' ...
Message from Schedule hook: [{"binding":"every 5 sec"}]
INFO     : TASK_RUN HookRun@SCHEDULE schedule-hook.sh
INFO     : Running hook 'schedule-hook.sh' binding 'SCHEDULE' ...
Message from Schedule hook: [{"binding":"every 10 sec"}]
...
```

### cleanup

```
$ kubectl delete ns/example-schedule
$ docker rmi registry.mycompany.com/shell-operator:schedule
```
