# shell-operator

Run your custom cluster-wide scripts in reaction to Kubernetes events or on schedule.

## Getting started

Build image with your hooks

FROM: flant/shell-operator:..
ADD: ....  

push image and create a Deployment



## Installation

```
kubectl apply scripts/shell-operator.yml
```

## Usage

```
mkdir shell-operator-config && cd shell-operator-config
git init
mkdir 001-first-hook

docker run --rm flant/shell-operator:latest mkconfigmap > shell-operator-cm.yml

kubectl apply shell-operator-cm.yml

```

## Hook

Hook is a script that will be executed on some event in Kubernetes cluster.
 
### hook configuration

Shell-operator on startup run every hook with `--config` argument.
Hook should return a json with configuration on stdout. Configuration
 

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

