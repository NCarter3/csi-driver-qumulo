## Driver Parameters
> This plugin driver itself only provides a communication layer between resources in the cluser and the Qumulo cluster, you need to bring your own Qumulo cluster before using this driver.

### Storage Class Usage (Dynamic Provisioning)
> [`StorageClass` example](../deploy/example/storageclass-qumulo.yaml)

Name | Meaning | Example Value | Mandatory | Default value | Notes
--- | --- | --- | --- | ---
server | Cluster name or IP | Domain name `cluster1.local` <br>Or IP address `4.5.6.7` | Yes | - |
storeRealPath | Directory on the cluster where volumes are stored | `/csi/volumes` | Yes | - | This
directory must exist and be writable by the configured user
storeExportPath | Export path pods will use to access volumes | `/share1` | No | `/` | The FS path
the export points to must be a prefix of storeRealPath.
restPort | Port used to talk to cluster | 8888 | No | 8000 | Useful for port forwarding or testing

### PV/PVC Usage (Static Provisioning)
> [`PersistentVolume` example](../deploy/example/pv-nfs-csi.yaml)

Name | Meaning | Example Value | Mandatory | Default value
--- | --- | --- | --- | ---
volumeAttributes.server | NFS Server endpoint | Domain name `nfs-server.default.svc.cluster.local` <br>Or IP address `127.0.0.1` | Yes |
volumeAttributes.share | NFS share path | `/` |  Yes  |
