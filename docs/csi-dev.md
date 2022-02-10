# NFS CSI driver development guide

## How to build this project
 - Clone repo
```console
$ mkdir -p $GOPATH/src/github.com
$ git clone https://github.com/ScottUrban/csi-driver-qumulo $GOPATH/src/github.com/csi-driver-qumulo
```

 - Build CSI driver
```console
$ cd $GOPATH/src/github.com/csi-driver-qumulo
$ make
```

 - Run verification test before submitting code
```console
$ make verify
```

 - If there is config file changed under `charts` directory, run following command to update chart file
```console
helm package charts/latest/csi-driver-qumulo -d charts/latest/
```

## How to test CSI driver in local environment

Install `csc` tool according to https://github.com/rexray/gocsi/tree/master/csc
```console
$ mkdir -p $GOPATH/src/github.com
$ cd $GOPATH/src/github.com
$ git clone https://github.com/rexray/gocsi.git
$ cd rexray/gocsi/csc
$ make build
```

#### Start CSI driver locally
```console
$ cd $GOPATH/src/github.com/csi-driver-qumulo
$ ./bin/qumuloplugin --endpoint tcp://127.0.0.1:10000 --nodeid CSINode -v=5 &
```

#### Have a Qumulo cluster to test against

This csi driver talks to a Qumulo cluster that provides the NFS storage for persisten volumes.
You need a cluster for testing with a user with sufficient rights, a directory
for volume storage, and an NFS export that covers that directory.

You can use these environment variable to refer to the cluster:
```console
$ QUMULO_TEST_HOST=localhost
$ QUMULO_TEST_PORT=18154
$ QUMULO_TEST_USERNAME=admin
$ QUMULO_TEST_PASSWORD=password
```
These same environment variables are used by the unit test and integration tests.

#### 0. Set other environment variables
```console
$ VOLNAME="test-$(date +%s)"
$ VOLSIZE=2147483648
$ ENDPOINT=tcp://127.0.0.1:10000
$ TARGET_PATH=/tmp/targetpath
$ STORE_PATH=/volumes
$ PARAMS="server=$QUMULO_TEST_HOST,restport=$QUMULO_TEST_PORT,storerealpath=$STORE_PATH"
# This one is exported for use by csc, others are just shell parameters.
$ export X_CSI_SECRETS="username=$QUMULO_TEST_USERNAME,password=$QUMULO_TEST_PASSWORD"
```

#### 1. Get plugin info
```console
$ csc identity plugin-info --endpoint $ENDPOINT
"qumulo.csi.k8s.io"     "v1.0.0"
```

#### 2. Create a new Qumulo volume
```console
$ value=$(csc controller create-volume --endpoint $ENDPOINT --cap 1,mount, --req-bytes $VOLSIZE --params $PARAMS $VOLNAME)
$ VOLUMEID=$(echo $value | awk '{print $1}' | sed 's/"//g')
$ echo "Got volume id: $VOLUMEID"
```

#### 3. Publish a Qumulo volume
Note: this doesn't work
```
$ csc node publish --endpoint $ENDPOINT --cap 1,mount, --vol-context "$PARAMS" --target-path "$TARGET_PATH" "$VOLUMEID"
```

#### 4. Unpublish a Qumulo volume
Note: this doesn't work
```console
$ csc node unpublish --endpoint $ENDPOINT --target-path "$TARGET_PATH" "$VOLUMEID"
```

#### 6. Validate volume capabilities
```console
$ csc controller validate-volume-capabilities --endpoint $ENDPOINT --cap 1,mount, "$VOLUMEID"
```

#### 7. Delete the Qumulo volume
```console
$ csc controller del --endpoint $ENDPOINT "$VOLUMEID" --timeout 10m
```

#### 8. Get NodeID
```console
$ csc node get-info --endpoint $ENDPOINT
CSINode
```

## How to test CSI driver in a Kubernetes cluster
- Set environment variable
```console
export REGISTRY=<dockerhub-alias>
export IMAGE_VERSION=latest
```

- Build continer image and push image to dockerhub
```console
# run `docker login` first
# build docker image
make container
# push the docker image
make push
```

- Deploy a Kubernetes cluster and make sure `kubectl get nodes` works on your dev box.

- Run E2E test on the Kubernetes cluster.

```console
# install Qumulo CSI Driver on the Kubernetes cluster
make e2e-bootstrap

# run the E2E test
make e2e-test
```
