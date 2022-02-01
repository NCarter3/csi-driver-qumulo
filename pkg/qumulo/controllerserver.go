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
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"context"
	"github.com/blang/semver"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"k8s.io/klog/v2"
)

// XXX scott:
// o use better version of semver than blang
// o GetCapacity
// o add copyright to all files
// o cache connections? 1 user at a time - could use auth file too

type ControllerServer struct {
	Driver *Driver
}

func createConnection(server string, restPort int, secrets map[string]string) (*Connection, error) {
	username := secrets["username"]
	password := secrets["password"]

	if username == "" || password == "" {
		return nil, status.Error(codes.Unauthenticated, "username and password secrets missing")
	}

	c := MakeConnection(server, restPort, username, password, new(http.Client))

	versionInfo, err := c.GetVersionInfo()
	if err != nil {
		return nil, err
	}

	version, err := versionInfo.GetSemanticVersion()
	if err != nil {
		return nil, err
	}

	minimumVersion := semver.Version{Major: 4, Minor: 2, Patch: 4}

	if version.LT(minimumVersion) {
		return nil, status.Errorf(
			codes.FailedPrecondition, "Cluster version %v must be >= %v", version, minimumVersion,
		)
	}

	return &c, nil
}

func transFormRestError(err error, transforms map[int]error) error {
	switch err.(type) {
	case RestError:
		z := err.(RestError)
		for handledStatus, handledErr := range transforms {
			if z.StatusCode == handledStatus {
				return handledErr
			}
		}
		return status.Errorf(codes.Internal, "Unhandled Error: %v", err.Error())
	}

	return err
}

type CreateParams struct {
	server          string
	restPort        int
	storeRealPath   string
	storeExportPath string
	name            string
}

// An internal representation of a volume created by the provisioner.
type qumuloVolume struct {
	// Volume id
	id string

	// Address of the cluster (paramServer).
	server string

	// REST API port on cluster (paramRestPort).
	restPort int

	// Directory where volume is stored.
	storeRealPath string

	// Mount path where volume is stored.
	storeMountPath string

	// Volume name (directory name created under storeRealPath and storeMountPath) - from req name.
	name string
}

func getQuotaLimit(capacityRange *csi.CapacityRange) (uint64, error) {
	if capacityRange == nil {
		return 0, status.Error(codes.InvalidArgument, "CapacityRange must be provided")
	}

	if bytes := capacityRange.GetRequiredBytes(); bytes != 0 {
		if bytes < 0 {
			return 0, status.Error(codes.InvalidArgument, "RequiredBytes must be positive")
		}
		return uint64(bytes), nil
	}

	if bytes := capacityRange.GetLimitBytes(); bytes != 0 {
		if bytes < 0 {
			return 0, status.Error(codes.InvalidArgument, "LimitBytes must be positive")
		}
		return uint64(bytes), nil
	}

	return 0, status.Error(codes.InvalidArgument, "RequiredBytes or LimitBytes must be provided")
}

// CreateVolume create a volume
func (cs *ControllerServer) CreateVolume(
	ctx context.Context,
	req *csi.CreateVolumeRequest,
) (*csi.CreateVolumeResponse, error) {
	name := req.GetName()

	if len(name) == 0 {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume name must be provided")
	}

	if err := cs.validateVolumeCapabilities(req.GetVolumeCapabilities()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	quotaLimit, err := getQuotaLimit(req.GetCapacityRange())
	if err != nil {
		return nil, err
	}

	// To get this feature, we need some kind of clone in the product.
	if req.GetVolumeContentSource() != nil {
		return nil, status.Error(codes.InvalidArgument, "Volume source unsupported")
	}

	params, err := newCreateParams(name, req.GetParameters())
	if err != nil {
		return nil, err
	}

	connection, err := createConnection(params.server, params.restPort, req.GetSecrets())
	if err != nil {
		return nil, err
	}

	qVol, err := newQumuloVolume(params, connection)
	if err != nil {
		return nil, err
	}

	attributes, err := connection.EnsureDir(qVol.storeRealPath, qVol.name)
	if err != nil {
		return nil, transFormRestError(
			err,
			map[int]error{
				404: status.Errorf(
					codes.NotFound,
					"%s directory %q missing for volume %q",
					paramStoreRealPath,
					qVol.storeRealPath,
					qVol.id,
				),
				409: status.Errorf(
					codes.AlreadyExists,
					"A non-directory entity exists at %q for volume %q",
					qVol.storeRealPath+"/"+qVol.name,
					qVol.id,
				),
			},
		)
	}

	err = connection.EnsureQuota(attributes.Id, quotaLimit)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"Failed to set quota on %v: %v",
			qVol.id,
			err.Error(),
		)
	}

	attributes, err = connection.FileChmod(attributes.Id, "0777")
	if err != nil {
		return nil, err
	}

	return &csi.CreateVolumeResponse{Volume: qVol.qumuloVolumeToCSIVolume()}, nil
}

func (cs *ControllerServer) DeleteVolume(
	ctx context.Context,
	req *csi.DeleteVolumeRequest,
) (*csi.DeleteVolumeResponse, error) {

	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}

	qVol, err := makeQumuloVolumeFromID(volumeID)
	if err != nil {
		// An invalid ID should be treated as doesn't exist
		klog.Warningf("failed to get Qumulo volume for volume id %v deletion: %v", volumeID, err)
		return &csi.DeleteVolumeResponse{}, nil
	}

	connection, err := createConnection(qVol.server, qVol.restPort, req.GetSecrets())
	if err != nil {
		return nil, err
	}

	path := qVol.getVolumeRealPath()
	klog.V(2).Infof("Removing subdirectory at %v with tree delete", path)

	err = connection.TreeDeleteCreate(path)
	if err != nil {
		return nil, transFormRestError(err, map[int]error{})
	}

	return &csi.DeleteVolumeResponse{}, nil
}

func (cs *ControllerServer) ControllerPublishVolume(
	ctx context.Context,
	req *csi.ControllerPublishVolumeRequest,
) (*csi.ControllerPublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *ControllerServer) ControllerUnpublishVolume(
	ctx context.Context,
	req *csi.ControllerUnpublishVolumeRequest,
) (*csi.ControllerUnpublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *ControllerServer) ControllerGetVolume(
	ctx context.Context,
	req *csi.ControllerGetVolumeRequest,
) (*csi.ControllerGetVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *ControllerServer) ValidateVolumeCapabilities(
	ctx context.Context,
	req *csi.ValidateVolumeCapabilitiesRequest,
) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if req.GetVolumeCapabilities() == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capabilities missing in request")
	}

	// supports all AccessModes, no need to check capabilities here
	return &csi.ValidateVolumeCapabilitiesResponse{Message: ""}, nil
}

func (cs *ControllerServer) ListVolumes(
	ctx context.Context,
	req *csi.ListVolumesRequest,
) (*csi.ListVolumesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *ControllerServer) GetCapacity(
	ctx context.Context,
	req *csi.GetCapacityRequest,
) (*csi.GetCapacityResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ControllerGetCapabilities implements the default GRPC callout.
// Default supports all capabilities
func (cs *ControllerServer) ControllerGetCapabilities(
	ctx context.Context,
	req *csi.ControllerGetCapabilitiesRequest,
) (*csi.ControllerGetCapabilitiesResponse, error) {
	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: cs.Driver.cscap,
	}, nil
}

func (cs *ControllerServer) CreateSnapshot(
	ctx context.Context,
	req *csi.CreateSnapshotRequest,
) (*csi.CreateSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *ControllerServer) DeleteSnapshot(
	ctx context.Context,
	req *csi.DeleteSnapshotRequest,
) (*csi.DeleteSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *ControllerServer) ListSnapshots(
	ctx context.Context,
	req *csi.ListSnapshotsRequest,
) (*csi.ListSnapshotsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *ControllerServer) ControllerExpandVolume(
	ctx context.Context,
	req *csi.ControllerExpandVolumeRequest,
) (*csi.ControllerExpandVolumeResponse, error) {

	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}

	qVol, err := makeQumuloVolumeFromID(volumeID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "Volume not found %q", volumeID)
	}

	quotaLimit, err := getQuotaLimit(req.GetCapacityRange())
	if err != nil {
		return nil, err
	}

	connection, err := createConnection(qVol.server, qVol.restPort, req.GetSecrets())
	if err != nil {
		return nil, err
	}

	attributes, err := connection.LookUp(qVol.getVolumeRealPath())
	if err != nil {
		return nil, transFormRestError(
			err,
			map[int]error{
				404: status.Errorf(codes.NotFound, "Directory for volume %q is missing", qVol.id),
			},
		)
	}

	err = connection.EnsureQuota(attributes.Id, quotaLimit)
	if err != nil {
		return nil, transFormRestError(
			err,
			map[int]error{
				404: status.Errorf(codes.NotFound, "Directory for volume %q is missing", qVol.id),
			},
		)
	}

	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes:         int64(quotaLimit),
		NodeExpansionRequired: false,
	}, nil
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

func newCreateParams(name string, params map[string]string) (*CreateParams, error) {
	var (
		server          string
		storeRealPath   string
		storeExportPath string
		restPort        int
		err             error
	)

	// Default cluster rest port
	restPort = 8000

	// Validate parameters (case-insensitive).
	// TODO do more strict validation.
	for k, v := range params {
		switch strings.ToLower(k) {
		case paramServer:
			server = v
		case paramStoreRealPath:
			storeRealPath = v
		case paramStoreExportPath:
			storeExportPath = v
		case paramRestPort:
			restPort, err = strconv.Atoi(v)
			if err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "invalid port %q", v)
			}
		default:
			return nil, status.Errorf(codes.InvalidArgument, "invalid parameter %q", k)
		}
	}

	if server == "" {
		return nil, status.Errorf(codes.InvalidArgument, "%s is a required parameter", paramServer)
	}

	if storeRealPath == "" {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"%s is a required parameter",
			paramStoreRealPath,
		)
	}
	if !strings.HasPrefix(storeRealPath, "/") {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"%s (%q) must start with a '/'",
			paramStoreRealPath,
			storeRealPath,
		)
	}
	storeRealPath = strings.TrimRight(storeRealPath, "/")

	if storeExportPath == "" {
		storeExportPath = "/"
	}

	if !strings.HasPrefix(storeExportPath, "/") {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"%s (%q) must start with a '/'",
			paramStoreExportPath,
			storeExportPath,
		)
	}

	storeExportPath = strings.TrimRight(storeExportPath, "/")

	re := regexp.MustCompile("(///*)")
	storeRealPath = re.ReplaceAllLiteralString(storeRealPath, "/")
	storeExportPath = re.ReplaceAllLiteralString(storeExportPath, "/")

	if storeRealPath == "" {
		storeRealPath = "/"
	}
	if storeExportPath == "" {
		storeExportPath = "/"
	}

	ret := &CreateParams{
		server:          server,
		restPort:        restPort,
		storeRealPath:   storeRealPath,
		storeExportPath: storeExportPath,
		name:            name,
	}

	return ret, nil
}

// Volume ID formats:
// v1:server:restPort//storeRealPath//storeMountPath//name

func newQumuloVolume(params *CreateParams, connetion *Connection) (*qumuloVolume, error) {

	export, err := connetion.ExportGet(params.storeExportPath)
	if err != nil {
		return nil, transFormRestError(
			err,
			map[int]error{
				404: status.Errorf(codes.NotFound, "Export %q not found", params.storeExportPath),
			},
		)
	}

	if !strings.HasPrefix(params.storeRealPath, export.FsPath) {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"Volume directory %q would not be accessible via export %q fs_path %q",
			params.storeRealPath,
			params.storeExportPath,
			export.FsPath,
		)
	}

	suffix := strings.TrimPrefix(params.storeRealPath, export.FsPath)
	mountPath := filepath.Join(params.storeExportPath, suffix)

	id := "v1:" + params.server + ":" + strconv.Itoa(params.restPort) +
		"/" + params.storeRealPath + "/" + mountPath + "//" + params.name

	vol := &qumuloVolume{
		id:             id,
		server:         params.server,
		restPort:       params.restPort,
		storeRealPath:  params.storeRealPath,
		storeMountPath: mountPath,
		name:           params.name,
	}

	return vol, nil
}

func (vol *qumuloVolume) getVolumeRealPath() string {
	return filepath.Join(vol.storeRealPath, vol.name)
}

func (vol *qumuloVolume) getVolumeSharePath() string {
	return filepath.Join(vol.storeMountPath, vol.name)
}

func (vol *qumuloVolume) qumuloVolumeToCSIVolume() *csi.Volume {
	return &csi.Volume{
		CapacityBytes: 0, // by setting it to zero, Provisioner uses PVC requested size as PV size
		VolumeId:      vol.id,
		VolumeContext: map[string]string{
			paramServer: vol.server,
			paramShare:  vol.getVolumeSharePath(),
		},
	}
}

func makeQumuloVolumeFromID(id string) (*qumuloVolume, error) {
	volRegex := regexp.MustCompile("^v1:([^:]+):([0-9]+)//(.*)//(.*)//([^/]+)$")
	tokens := volRegex.FindStringSubmatch(id)
	if tokens == nil {
		return nil, fmt.Errorf("Could not decode volume ID %q", id)
	}

	restPort, err := strconv.Atoi(tokens[2])
	if err != nil {
		return nil, fmt.Errorf("Invalid port in volume ID %q", id)
	}

	return &qumuloVolume{
		id:             id,
		server:         tokens[1],
		restPort:       restPort,
		storeRealPath:  "/" + tokens[3],
		storeMountPath: "/" + tokens[4],
		name:           tokens[5],
	}, nil
}
