/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package qumulo

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"k8s.io/klog/v2"
)

// XXX scott:
// o add copyright to all files
// o cache connections? 1 user at a time - could use auth file too
// o error type/code for fmt.Errorf uses
// o use req capacity for quota, if not zero
// o gen exports? so we get capacity locally in pods with fsstat
// o relative exports and stuff - assume / access for now

// ControllerServer controller server setting
type ControllerServer struct {
	Driver *Driver
}

func createConnection(server string, restPort int, secrets map[string]string) (*Connection, error) {
	username := secrets["username"]
	password := secrets["password"]

	if username == "" || password == "" {
		return nil, status.Error(codes.Unauthenticated, "username and password secrets missing")
	}

	c := MakeConnection(server, restPort, username, password)

	return &c, nil
}

// An internal representation of a volume created by the provisioner.
type qumuloVolume struct {
	// Volume id
	id string

	// Address of the cluster (paramServer).
	server string

	// REST API port on cluster (paramRestPort).
	restPort int

	// Base directory on the cluster to create volumes under (paramShare).
	baseDir string

	// Volume direcotry (from req name)
	subDir string

	// size of volume (from req capacity)
	size int64
}

// CreateVolume create a volume
func (cs *ControllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	name := req.GetName()

	klog.Infof("SCOTT CREATE: %s", name)

	if len(name) == 0 {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume name must be provided")
	}
	if err := cs.validateVolumeCapabilities(req.GetVolumeCapabilities()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	reqCapacity := req.GetCapacityRange().GetRequiredBytes()

	klog.Infof("SCOTT req %v", req)

	qVol, err := cs.newQumuloVolume(name, reqCapacity, req.GetParameters())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	var volCap *csi.VolumeCapability
	if len(req.GetVolumeCapabilities()) > 0 {
		volCap = req.GetVolumeCapabilities()[0]
	}
	klog.Infof("SCOTT volCap %v", volCap)

	secrets := req.GetSecrets()
	klog.Infof("SCOTT secrets %v", secrets)

	connection, err := createConnection(qVol.server, qVol.restPort, secrets)
	if err != nil {
		return nil, err
	}

	// XXX scott: basedir
	id, err := connection.CreateDir("/", name)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create volume dir: %v", err.Error())
	}

	// XXX scott: this can overflow? stupid golang
	err = connection.CreateQuota(id, uint64(reqCapacity))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to set quota on ... : %v", err.Error())
	}


	// XXX scott: handle exists

	// XXX scott: chmod 0777 new directory?

	return &csi.CreateVolumeResponse{Volume: cs.qumuloVolumeToCSIVolume(qVol)}, nil
}

// DeleteVolume delete a volume
func (cs *ControllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is empty")
	}
	klog.Infof("SCOTT DELETE: %s", volumeID)
	qVol, err := cs.getQumuloVolumeFromID(volumeID)
	if err != nil {
		// An invalid ID should be treated as doesn't exist
		klog.Warningf("failed to get Qumulo volume for volume id %v deletion: %v", volumeID, err)
		return &csi.DeleteVolumeResponse{}, nil
	}

	secrets := req.GetSecrets()
	klog.Infof("SCOTT secrets %v", secrets)

	// XXX scott rest port
	connection, err := createConnection(qVol.server, 18154, secrets)
	if err != nil {
		return nil, err
	}

	path := cs.getVolumeSharePath(qVol)
	klog.V(2).Infof("Removing subdirectory at %v with tree delete", path)

	// XXX scott: in function, allow ENOENT in resolve and delete
	err = connection.TreeDeleteCreate(path)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to tree delete subdirectory: %v", err.Error())
	}

	return &csi.DeleteVolumeResponse{}, nil
}

func (cs *ControllerServer) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *ControllerServer) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *ControllerServer) ControllerGetVolume(ctx context.Context, req *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *ControllerServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if req.GetVolumeCapabilities() == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capabilities missing in request")
	}

	// supports all AccessModes, no need to check capabilities here
	return &csi.ValidateVolumeCapabilitiesResponse{Message: ""}, nil
}

func (cs *ControllerServer) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *ControllerServer) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ControllerGetCapabilities implements the default GRPC callout.
// Default supports all capabilities
func (cs *ControllerServer) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: cs.Driver.cscap,
	}, nil
}

func (cs *ControllerServer) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *ControllerServer) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *ControllerServer) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *ControllerServer) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {

	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is empty")
	}

	klog.Infof("SCOTT EXPAND: %s", volumeID)

	qVol, err := cs.getQumuloVolumeFromID(volumeID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "Volume not found %q", volumeID)
	}

	reqCapacity := req.GetCapacityRange().GetRequiredBytes()
	klog.Infof("SCOTT reqCapacity %v", reqCapacity)

	secrets := req.GetSecrets()
	klog.Infof("SCOTT secrets %v", secrets)

	connection, err := createConnection(qVol.server, qVol.restPort, secrets)
	if err != nil {
		return nil, err
	}

	// XXX scott: basedir
	id, err := connection.ResolvePath("/" + qVol.subDir)
	if err != nil {
		return nil, err // XXX
	}

	// XXX scott: this can overflow? stupid golang
	err = connection.UpdateQuota(id, uint64(reqCapacity))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to set quota on ... : %v", err.Error())
	}

	// XXX ExpandInUsePersistentVolumes required somewhere

	//return nil, status.Error(codes.Unimplemented, "")
	return &csi.ControllerExpandVolumeResponse{CapacityBytes: reqCapacity, NodeExpansionRequired: false}, nil
}

func (cs *ControllerServer) validateVolumeCapabilities(caps []*csi.VolumeCapability) error {
	if len(caps) == 0 {
		return fmt.Errorf("volume capabilities must be provided")
	}

	for _, c := range caps {
		if err := cs.validateVolumeCapability(c); err != nil {
			return err
		}
	}
	return nil
}

func (cs *ControllerServer) validateVolumeCapability(c *csi.VolumeCapability) error {
	if c == nil {
		return fmt.Errorf("volume capability must be provided")
	}

	// Validate access mode
	accessMode := c.GetAccessMode()
	if accessMode == nil {
		return fmt.Errorf("volume capability access mode not set")
	}
	if !cs.Driver.cap[accessMode.Mode] {
		return fmt.Errorf("driver does not support access mode: %v", accessMode.Mode.String())
	}

	// Validate access type
	accessType := c.GetAccessType()
	if accessType == nil {
		return fmt.Errorf("volume capability access type not set")
	}
	return nil
}

// Volume ID formats:
// v1:server:restPort//baseDir//name

func (cs *ControllerServer) newQumuloVolume(name string, size int64, params map[string]string) (*qumuloVolume, error) {
	var (
		server               string
		baseDir              string
		restPort             int
		err                  error
	)

	// Default cluster rest port
	restPort = 8000

	// Validate parameters (case-insensitive).
	// TODO do more strict validation.
	for k, v := range params {
		switch strings.ToLower(k) {
		case paramServer:
			server = v
		case paramShare:
			baseDir = v
		case paramRestPort:
			restPort, err = strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("invalid port %q", v)
			}
		default:
			return nil, fmt.Errorf("invalid parameter %q", k)
		}
	}

	// Validate required parameters
	if server == "" {
		return nil, fmt.Errorf("%v is a required parameter", paramServer)
	}
	if baseDir == "" {
		return nil, fmt.Errorf("%v is a required parameter", paramShare)
	}

	id := "v1:" + server + ":" + strconv.Itoa(restPort) + "//" + baseDir + "//" + name

	vol := &qumuloVolume{
		id:                   id,
		server:               server,
		restPort:             restPort,
		baseDir:              baseDir,
		subDir:               name,
		size:                 size,
	}

	return vol, nil
}

// Get user-visible share path for the volume
func (cs *ControllerServer) getVolumeSharePath(vol *qumuloVolume) string {
	return filepath.Join(string(filepath.Separator), vol.baseDir, vol.subDir)
}

func (cs *ControllerServer) qumuloVolumeToCSIVolume(vol *qumuloVolume) *csi.Volume {
	return &csi.Volume{
		CapacityBytes: 0, // by setting it to zero, Provisioner will use PVC requested size as PV size
		VolumeId:      vol.id,
		VolumeContext: map[string]string{
			paramServer: vol.server,
			paramShare:  cs.getVolumeSharePath(vol),
		},
	}
}

func (cs *ControllerServer) getQumuloVolumeFromID(id string) (*qumuloVolume, error) {
	volRegex := regexp.MustCompile("^v1:([^:]+):([0-9]+)//(.*)//([^/]+)$")
	tokens := volRegex.FindStringSubmatch(id)
	if tokens == nil {
		return nil, fmt.Errorf("Could not decode volume ID %q", id)
	}

	restPort, err := strconv.Atoi(tokens[2])
	if err != nil {
		return nil, fmt.Errorf("invalid port %q", id)
	}

	return &qumuloVolume{
		id:       id,
		server:   tokens[1],
		restPort: restPort,
		baseDir:  tokens[3],
		subDir:   tokens[4],
	}, nil
}
