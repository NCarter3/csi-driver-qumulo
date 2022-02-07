## Driver Parameters
> This plugin driver itself only provides a communication layer between resources in the cluser and the Qumulo cluster, you need to bring your own Qumulo cluster before using this driver.

### Storage Class Usage (Dynamic Provisioning)
> [`StorageClass` example](../deploy/example/storageclass-qumulo.yaml)

Name | Meaning | Example Value | Mandatory | Default
--- | --- | --- | --- | ---
server | Qumulo cluster name or IP | `cluster1` <br>Or `4.5.6.7` | Yes |
storeRealPath | Directory volumes are stored | `/csi/volumes` | Yes |
storeExportPath | Export used to access volumes | `/share1` | No | `/` | The FS path the export points to must be a prefix of storeRealPath.
restPort | Qumulo cluster rest port | 8888 | No | 8000
csi.storage.k8s.io/provisioner-secret-name | Credentials | cluster1-login | Yes |
csi.storage.k8s.io/provisioner-secret-namespace | Credentials | kube-system | Yes |
csi.storage.k8s.io/controller-expand-secret-name | Credentials | cluster1-login | Yes |
csi.storage.k8s.io/controller-expand-secret-namespace | Credentials | kube-system | Yes |

The *storeRealPath* directory must exist on the Qumulo cluster and be writable by the configured user.

The *storeExportPath* export must with exist with an `FS Path` which is partial or full prefix of the storeRealPath.

#### Qumulo Cluster Login Parameters

- csi.storage.k8s.io/provisioner-secret-name: cluster1-login
- csi.storage.k8s.io/provisioner-secret-namespace: kube-system
- csi.storage.k8s.io/controller-expand-secret-name: cluster1-login
- csi.storage.k8s.io/controller-expand-secret-namespace: kube-system

These two pairs of parameters specify the secret name and secret namespace for
the secret which contains the username and password to talk to the Qumulo cluster
with. One set is used for volume creation and deletion and the second set is
used during volume expansion. It's recommended to use the same secret (and thus
the same user) for both sets of operations.

The configured username must have the following privileges to operate:

* Look up on `storeRealPath`
* Directory creation in `storeRealPath`
* Creating and modifying quotas (PRIVILEGE_QUOTA_READ)
* Reading NFS exports (PRIVILEGE_NFS_EXPORT_READ)
* TreeDelete of volume directories (PRIVILEGE_FS_DELETE_TREE_WRITE)

The `admin` user has all these rights, or you can use RBAC on the cluster to use another user.

An example of creating secrets for the username `bill` with password `SuperSecret`

```
% kubectl create secret generic cluster1-login --type="kubernetes.io/basic-auth" --from-literal=username=bill --from-literal=password=SuperSecret --namespace=kube-system
```

The `mountOptions` spec can be used to control how the node mounts the created volume.

### PV/PVC Usage (Static Provisioning)
> [`PersistentVolume` example](../deploy/example/static-pv.yaml)

The volume size specification (spec.capacity.storage) has no effect with static volumes.

Name | Meaning | Example Value | Mandatory | Default value
--- | --- | --- | --- | ---
volumeAttributes.server | NFS Server endpoint | `cluster1` <br>Or `127.0.0.1` | Yes |
volumeAttributes.share | NFS export path | `/` |  Yes  |

