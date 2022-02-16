Binding Context Generator
=========================
Binding Context Generator provides the ability to generate binding contexts for hooks testing purposes.

Usage example:
1. Hook Config
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
2. Initial objects state
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
3. New state
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
c := context.NewBindingContextController(config, initialState)
```
5. Register CRDs (if it is necessary) in format [Group, Version, Kind, isNamespaced]:
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
7. Change state to get new binding contexts
```go
contexts, err = c.ChangeState(newState)
if err != nil {
  return err
}
testNewContexts(contexts)
```
8. Run schedule to get binding contexts for a cron task
```go
contexts, err = c.RunSchedule("* * * * *")
if err != nil {
  return err
}
testScheduleContexts(contexts)
```
