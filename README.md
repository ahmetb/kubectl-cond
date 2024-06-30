# kubectl cond

A kubectl plugin to print Kubernetes object resource conditions.

## Usage

```shell
kubectl cond <object-type> <object-name>
```

## Example

```text
kubectl cond node node-1

+----------------+----------------------------------------------------------------+
| CONDITION TYPE |                            DETAILS                             |
+----------------+----------------------------------------------------------------+
| Ready          | KubeletReady                                                   |
| (True)         | (kubelet is posting ready status)                              |
|                | * Last Transition: 2024-05-12T08:20:20-07:00 (1180h59m55s ago) |
+----------------+----------------------------------------------------------------+
| MemoryPressure | KubeletHasSufficientMemory                                     |
| (False)        | (kubelet has sufficient memory available)                      |
|                | * Last Transition: 2024-05-12T08:20:20-07:00 (1180h59m55s ago) |
+----------------+----------------------------------------------------------------+
| DiskPressure   | KubeletHasNoDiskPressure                                       |
| (False)        | (kubelet has no disk pressure)                                 |
|                | * Last Transition: 2024-05-12T08:20:20-07:00 (1180h59m55s ago) |
+----------------+----------------------------------------------------------------+
| PIDPressure    | KubeletHasSufficientPID                                        |
| (False)        | (kubelet has sufficient PID available)                         |
|                | * Last Transition: 2024-05-12T08:20:20-07:00 (1180h59m55s ago) |
+----------------+----------------------------------------------------------------+
```
