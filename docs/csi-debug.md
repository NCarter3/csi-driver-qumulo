## CSI driver debug tips

### Case#1: volume create/delete failed
 - locate csi driver pod
```console
$ kubectl get pod -o wide -n kube-system | grep csi-qumulo-controller
NAME                                     READY   STATUS    RESTARTS   AGE     IP             NODE
csi-qumulo-controller-56bfddd689-dh5tk      5/5     Running   0          35s     10.240.0.19    k8s-agentpool-22533604-0
csi-qumulo-controller-56bfddd689-sl4ll      5/5     Running   0          35s     10.240.0.23    k8s-agentpool-22533604-1
```
 - get csi driver logs
```console
$ kubectl logs csi-qumulo-controller-56bfddd689-dh5tk -c qumulo -n kube-system > csi-qumulo-controller.log
```
> note: there could be multiple controller pods, if there are no helpful logs, try to get logs from other controller pods

### Case#2: volume mount/unmount failed
 - locate csi driver pod and figure out which pod does tha actual volume mount/unmount

```console
$ kubectl get pod -o wide -n kube-system | grep csi-qumulo-node
NAME                                      READY   STATUS    RESTARTS   AGE     IP             NODE
csi-qumulo-node-cvgbs                        3/3     Running   0          7m4s    10.240.0.35    k8s-agentpool-22533604-1
csi-qumulo-node-dr4s4                        3/3     Running   0          7m4s    10.240.0.4     k8s-agentpool-22533604-0
```

 - get csi driver logs
```console
$ kubectl logs csi-qumulo-node-cvgbs -c qumulo -n kube-system > csi-qumulo-node.log
```

 - check qumulo mount inside driver
```console
kubectl exec -it csi-qumulo-node-cvgbss -n kube-system -c qumulo -- mount | grep nfs
```

### troubleshooting connection failure on agent node
```console
mkdir /tmp/test
mount -v -t nfs -o ... nfs-server:/path /tmp/test
```
