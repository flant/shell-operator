## build shell-operator from specific base image

shell-operator binary is statically linked, so you can use it to build your own shell-operator with specific set of utilities! This example shows how to build shell-operator from ubuntu:20.04 image.


### Run

Build shell-operator image with docker:

```
docker build . -t my-repo/shell-operator:latest

```

### Cleanup

```
docker rmi my-repo/shell-operator:latest
```
