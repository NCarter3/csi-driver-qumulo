# Install Qumulo CSI driver master version on a kubernetes cluster

*TODO: helm not yet working.*

If you have already installed Helm, you can also use it to install Qumulo CSI driver. Please see [Installation with Helm](../charts/README.md).

## Install with kubectl
 - remote install
```console
curl -skSL https://raw.githubusercontent.com/scotturban/csi-driver-qumulo/master/deploy/install-driver.sh | bash -s master --
```

 - local install
```console
git clone https://github.com/scotturban/csi-driver-qumulo.git
cd csi-driver-qumulo
deploy/install-driver.sh master local
# or 
deploy/install-driver.bat master local

```

- check pods status:
```console
kubectl -n kube-system get pod -o wide -l app=csi-qumulo-controller
kubectl -n kube-system get pod -o wide -l app=csi-qumulo-node
```

example output:

```console
NAME                                     READY   STATUS    RESTARTS   AGE     IP             NODE       NOMINATED NODE   READINESS GATES
csi-qumulo-controller-5fc9fc98cc-wbv8b   4/4     Running   0          5h29m   192.168.49.2   minikube   <none>           <none>

NAME                    READY   STATUS    RESTARTS   AGE     IP             NODE       NOMINATED NODE   READINESS GATES
csi-qumulo-node-84lvs   3/3     Running   0          5h29m   192.168.49.2   minikube   <none>           <none>
```

- clean up Qumulo CSI driver (remote)
```console
curl -skSL https://raw.githubusercontent.com/scotturban/csi-driver-qumulo/master/deploy/uninstall-driver.sh | bash -s master --
```

- clean up Qumulo CSI driver (local)
```
deploy/install-driver.sh master local
# or 
deploy/install-driver.bat master local
```

