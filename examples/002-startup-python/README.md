## onStartup python example

Example of a hook written in Python.

### run

Build shell-operator image with custom scripts:

```
$ docker build -t "registry.mycompany.com/shell-operator:startup-python" .
$ docker push registry.mycompany.com/shell-operator:startup-python
```

Edit image in shell-operator-pod.yaml and apply manifests:

```
$ kubectl create ns example-startup-python 
$ kubectl -n example-startup-python apply -f shell-operator-pod.yaml
```

See in logs that 00-hook.py was run:

```
$ kubectl -n example-startup-python logs -f po/shell-operator
...
INFO     : Running hook '00-hook.py' binding 'ON_STARTUP' ...
OnStartup Python powered hook
...
```

### cleanup

```
$ kubectl delete ns/example-startup-python
$ docker rmi registry.mycompany.com/shell-operator:startup-python
```
