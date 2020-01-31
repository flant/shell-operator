Binding Context Generator
=========================
Binding Context Generator allows to generate binding context values for hook testing purpose.

Usage example:
1. Declare the hook config
```go
config := `
configVersion: v1
schedule:
- name: every_minute
  crontab: '* * * * *'
  includeSnapshotsFrom:
  - pod
kubernetes:
- apiVersion: v1
  name: pod
  kind: Pod
  watchEvent:
  - Added
  - Modified
  - Deleted
  namespace:
    nameSelector:
      matchNames:
      - default`
```
2. Declare initial state of kubernetes objects
```go
initialState := `
---
apiVersion: v1
kind: Pod
metadata:
  name: pod-0
---
apiVersion: v1
kind: Pod
metadata:
  name: pod-1
`
```
3. Declare new state (one of pods are deleted)
```go
newState := `
---
apiVersion: v1
kind: Pod
metadata:
  name: pod-0
`
```
4. Create new binding context controller
```go
c, err := context.NewBindingContextController(config, initialState)
if err != nil {
  return err
}
```
5. Register CRD (if you use some in tests) in format [Group, Version, Kind, isNamespaced]:
```go
c.RegisterCRD("example.com", "v1", "Example", true)
c.RegisterCRD("example.com", "v1", "ClusterExample", false)
```
6. Run controller to get initial binding context
```go
contexts, err := c.Run()
if err != nil {
  return err
}
testContexts(contexts)
```
7. Change state to new get binding contexts
```go
contexts, err = c.ChangeState(newState)
if err != nil {
  return err
}
testNewContexts(contexts)
```
8. Run schedule to get binding contexts
```go
contexts, err = c.RunSchedule("* * * * *")
if err != nil {
  return err
}
testScheduleContexts(contexts)
```