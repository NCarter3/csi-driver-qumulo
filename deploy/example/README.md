# CSI driver example

After the Qumulo CSI Driver is deployed in your Kubernetes cluster, you can follow this documentation to quickly deploy some examples. 

You can use Qumulo CSI Driver to provision Persistent Volumes statically or dynamically. Please read [Kubernetes Persistent Volumes documentation](https://kubernetes.io/docs/concepts/storage/persistent-volumes/) for more information about Static and Dynamic provisioning.

Please refer to [driver parameters](../../docs/driver-parameters.md) for more detailed usage.

## Prerequisite

- A Qumulo cluster at version 4.2.4 or greater.
- [Install Qumulo CSI Driver](../../docs/install-qumulo-csi-driver.md)

## Storage Class Usage (Dynamic Provisioning)

- **storeRealPath**: You will need a directory on the Qumulo cluster where the driver stores volumes (e.g. `/csi/volumes`).

- **storeExportPath**: You will need an export on the Qumulo cluster which will be used by pods that want to mount the volumes. This export's FS-path must be a prefix of the *storeRealPath* (e.g. `/exports/csi` -> `/csi`)

- You will need a username and password for a user on the Qumulo cluster that can create directories in *storeRealPath*, run tree-delete, read exports, and create and modify quotas.

- Configure the secret for that username, e.g.;

```
% kubectl create secret generic cluster1-login --type="kubernetes.io/basic-auth" --from-literal=username=bill --from-literal=password=SuperSecret --namespace=kube-system
```

- Give the driver access to the secrets, e.g.:
```
% kubectl create role access-secrets --verb=get,list,watch,update,create --resource=secrets --namespace kube-system
 % kubectl create rolebinding --role=access-secrets default-to-secrets --serviceaccount=kube-system:csi-qumulo-controller-sa --namespace kube-system
```

- Create a `StorageClass`

  - Get configuration
    ```bash
    wget https://raw.githubusercontent.com/ScottUrban/csi-driver-qumulo/master/deploy/example/storageclass-qumulo.yaml
    ```

  - Edit the configuration for your Qumulo cluster
    - name your storage class
    - modify `server` and `storeRealPath`
    - modify `storeExportPath` or delete if you want to use a `/` export
    - modify the two sets of secret-name and secret-namespace parameters to point to your secret in the namespace where you installed the driver
    - modify mountOptions if needed. See [driver parameters](../../docs/driver-parameters.md) for more info.

  - Apply the configuration to create the class.
```
kubectl create -f storageclass-qumulo.yaml
```

- Create a `PersistentVolumeClaim` dynamically.

  - Get configuration
```
wget https://raw.githubusercontent.com/ScottUrban/csi-driver-qumulo/master/deploy/example/dynamic-pvc.yaml
```

  - Edit the configuration
    - name the claim
    - change the `storeClassName` to the name used above
    - change the capacity (`spec.resources.requests.storage`) - this will be used to create a quota on the Qumulo cluster. It can be increased later with ExpandVolume.

  - Apply the configuration to create the claim.
```
kubectl apply -f dyanmic-pvc.yaml
```

- Use the claim in a pod. See the Kubernetes documentation for more information, but you might have something like this:

```
---
apiVersion: v1
kind: Pod
metadata:
  name: claim1-pod
spec:
  volumes:
    - name: cluster1
      persistentVolumeClaim:
        claimName: claim1
  containers:
    - name: claim1-container
      image: ...
      volumeMounts:
        - mountPath: "/cluster1"
          name: cluster1
  ...
```

- The directory storing the volume will be removed with Tree Delete when the pvc is deleted. You can prevent this by changing the `reclaimPolicy` to `Retain`.

---

## PV/PVC Usage (Static Provisioning)

- Have an existing Qumulo cluster export you want to use as a volume

- Create `PersistentVolume` statically.

  - Get configuration
    ```bash
    wget https://raw.githubusercontent.com/ScottUrban/csi-driver-qumulo/master/deploy/example/static-pv.yaml
    ```

  -  Modify the configuration
     -  Give the volume a unique name
     -  Change the server to your Qumulo cluster domain name or IP
     -  Change the share to the Qumulo cluster export path
     -  Modify mountOptions if needed
    See [driver parameters](../../docs/driver-parameters.md) for more info.

  -  Apply the configuration to create the pv:
     ```
     kubectl apply -f static-pv.yaml
    ```

- Create the `PersistentVolumeClaim` statically.

  -  Get configuration
     ```
     wget https://raw.githubusercontent.com/ScottUrban/csi-driver-qumulo/master/deploy/example/static-pvc.yaml
     ```

  -  Modify the configuration
     - change the name
     - change the volume name to the name used above

  -  Apply the configuration to create the pvc:
     ```
     kubectl apply -f static-pvc.yaml
     ```

- Use the claim in your pods.
