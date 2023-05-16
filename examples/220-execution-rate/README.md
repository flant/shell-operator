# Example with conversion hooks

This is a simple example of execution rate settings. Read more information in [HOOKS.md](../../docs/src/HOOKS.md#execution-rate).

The example contains one hook that is subscribed to "crontab" resources and should be executed not more than once in 5 seconds.

## Run

### Build and install example

Build Docker image and use helm3 to install it:

```
docker build -t localhost:5000/shell-operator:example-220 .
docker push localhost:5000/shell-operator:example-220
helm upgrade --install \
    --namespace example-220 \
    --create-namespace \
    example-220 .
```

### See rate limited hook in action

1. Run a simple script to recreate "crontab" resources and thus emit multiple events:

```
$ ./crontab-recreate.sh
crontab.stable.example.com "crontab-obj-1" deleted
crontab.stable.example.com/crontab-obj-1 replaced
crontab.stable.example.com "crontab-obj-2" deleted
crontab.stable.example.com/crontab-obj-2 replaced
crontab.stable.example.com "crontab-obj-3" deleted
crontab.stable.example.com/crontab-obj-3 replaced
crontab.stable.example.com "crontab-obj-4" deleted
crontab.stable.example.com/crontab-obj-4 replaced
crontab.stable.example.com "crontab-obj-1" deleted
crontab.stable.example.com/crontab-obj-1 replaced
crontab.stable.example.com "crontab-obj-2" deleted
crontab.stable.example.com/crontab-obj-2 replaced
...
```

2. Now see what's in logs:

```
$ kubectl -n example-220 logs deploy/shell-operator -f | grep RateLimitedHook

{"binding":"crontabs","event":"kubernetes","hook":"settings-rate-limit.sh","level":"info","msg":"RateLimitedHook binding contexts: 18","output":"stdout","queue":"main","task":"HookRun","time":"2021-02-27T12:57:42Z"}
{"binding":"crontabs","event":"kubernetes","hook":"settings-rate-limit.sh","level":"info","msg":"RateLimitedHook binding contexts: 24","output":"stdout","queue":"main","task":"HookRun","time":"2021-02-27T12:57:47Z"}
{"binding":"crontabs","event":"kubernetes","hook":"settings-rate-limit.sh","level":"info","msg":"RateLimitedHook binding contexts: 24","output":"stdout","queue":"main","task":"HookRun","time":"2021-02-27T12:57:52Z"}
{"binding":"crontabs","event":"kubernetes","hook":"settings-rate-limit.sh","level":"info","msg":"RateLimitedHook binding contexts: 16","output":"stdout","queue":"main","task":"HookRun","time":"2021-02-27T12:57:57Z"}
{"binding":"crontabs","event":"kubernetes","hook":"settings-rate-limit.sh","level":"info","msg":"RateLimitedHook binding contexts: 24","output":"stdout","queue":"main","task":"HookRun","time":"2021-02-27T12:58:02Z"}
{"binding":"crontabs","event":"kubernetes","hook":"settings-rate-limit.sh","level":"info","msg":"RateLimitedHook binding contexts: 24","output":"stdout","queue":"main","task":"HookRun","time":"2021-02-27T12:58:07Z"}
```

Note that hook is executed every 5s as stated in its settings.

### Cleanup

```
helm delete --namespace=example-220 example-220
kubectl delete ns example-220
kubectl delete crd crontabs.stable.example.com
```
