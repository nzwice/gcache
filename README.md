# gcache

A distributed grow-only LRU cache using [hashicorp/memberlist](https://github.com/hashicorp/memberlist) with delta-based CRDT & anti-entropy (in the context of Kubernetes pods)

![Traffic Flow](/diagram.png)

## Usages

```
go run . -count=10
```

```
➜  ~ curl localhost:8098/
host=pod-#0, known=(1) [pod-#0], state=map[pod-#0:map[]]%                                                                   ➜  ~ curl localhost:8098/inc/a
host=pod-#1, key(a) = 1%                                                                                                    ➜  ~ curl localhost:8098/inc/a
host=pod-#0, key(a) = 2%                                                                                                    ➜  ~ curl localhost:8098/inc/a
host=pod-#6, key(a) = 3%                                                                                                    ➜  ~ curl localhost:8098/inc/a
host=pod-#2, key(a) = 4%                                                                                                    ➜  ~ curl localhost:8098/get/a
host=pod-#0, key(a) = 4%
```

## Command line arguments

```
Usage of /app/main:
  -count int
        the number of expected pods in the cluster (default 2)
  -delay int
        delay seconds before starting a pod, the ith pod will be delayed within (delay * i) seconds randomly (default 5)
  -jitter int
        extra jitter seconds to delay a pod (default 5)
  -lb string
        the load balancer http address (default ":8098")
  -port int
        the starting gossip port, the ith pod will listen gossip port on (starting + i) (default 8100)
  -report int
        periodic seconds for a pod to report its known cluster state (default 3)
```
