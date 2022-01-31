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
	"reflect"
	"strconv"
	"strings"
	"testing"

	"fmt"

	"context"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	mount "k8s.io/mount-utils"
)

const (
	testCSIVolume = "test-csi"
	testVolumeID  = "test-server/test-base-dir/test-csi"
)

func initTestController(t *testing.T) *ControllerServer {
	var perm *uint32
	mounter := &mount.FakeMounter{MountPoints: []mount.MountPoint{}}
	driver := NewDriver("", "", "", perm)
	driver.ns = NewNodeServer(driver, mounter)
	cs := NewControllerServer(driver)
	return cs
}

func teardown() {
	// XXX scott: did have mount removal - use in e2e tests? - see the fixture stuff in lib_test.go
}

func makeVolumeId(dirPath string, name string) string {
	return fmt.Sprintf("v1:%s:%d//%s////%s", testHost, testPort, strings.Trim(dirPath, "/"), name)
}

/*   ____                _     __     __    _
 *  / ___|_ __ ___  __ _| |_ __\ \   / /__ | |_   _ _ __ ___   ___
 * | |   | '__/ _ \/ _` | __/ _ \ \ / / _ \| | | | | '_ ` _ \ / _ \
 * | |___| | |  __/ (_| | ||  __/\ V / (_) | | |_| | | | | | |  __/
 *  \____|_|  \___|\__,_|\__\___| \_/ \___/|_|\__,_|_| |_| |_|\___|
 *  FIGLET: CreateVolume
 */

func makeCreateRequest(testDirPath string, name string) csi.CreateVolumeRequest {
	return csi.CreateVolumeRequest{
		Name: name,
		VolumeCapabilities: []*csi.VolumeCapability{
			{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{},
				},
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
				},
			},
		},
		CapacityRange: &csi.CapacityRange{RequiredBytes: 1024 * 1024 * 1024},
		Parameters: map[string]string{
			paramServer:         testHost,
			paramRestPort:       strconv.Itoa(testPort),
			paramStoreRealPath:  testDirPath,
			paramStoreMountPath: "/",
		},
		Secrets: map[string]string{
			"username": testUsername,
			"password": testPassword,
		},
	}
}

func makeCreateResponse(testDirPath string, name string) *csi.CreateVolumeResponse {
	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId: makeVolumeId(testDirPath, name),
			VolumeContext: map[string]string{
				paramServer: testHost,
				paramShare:  "/"+name,
			},
		},
	}
}

func TestCreateVolumeNameMissing(t *testing.T) {
	testDirPath, _, cleanup := requireCluster(t)
	defer cleanup(t)

	req := makeCreateRequest(testDirPath, "")

	_, err := initTestController(t).CreateVolume(context.TODO(), &req)

	assert.Equal(
		t,
		err,
		status.Error(codes.InvalidArgument, "CreateVolume name must be provided"),
	)
}

func TestCreateVolumeInvalidVolumeCapabilities(t *testing.T) {
	testDirPath, _, cleanup := requireCluster(t)
	defer cleanup(t)

	req := makeCreateRequest(testDirPath, "foobar")
	req.VolumeCapabilities = []*csi.VolumeCapability{
		{
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
			},
		},
	}

	_, err := initTestController(t).CreateVolume(context.TODO(), &req)

	assert.Equal(
		t,
		err,
		status.Error(codes.InvalidArgument, "volume capability access type not set"),
	)
}

func TestCreateVolumeInvalidCapacityRange(t *testing.T) {
	testDirPath, _, cleanup := requireCluster(t)
	defer cleanup(t)

	req := makeCreateRequest(testDirPath, "foobar")
	req.CapacityRange = nil

	_, err := initTestController(t).CreateVolume(context.TODO(), &req)

	assert.Equal(t, err, status.Error(codes.InvalidArgument, "CapacityRange must be provided"))
}

func TestCreateVolumeUnknownParameter(t *testing.T) {
	testDirPath, _, cleanup := requireCluster(t)
	defer cleanup(t)

	req := makeCreateRequest(testDirPath, "foobar")
	req.Parameters["wut"] = "ever"

	_, err := initTestController(t).CreateVolume(context.TODO(), &req)

	assert.Equal(t, err, status.Error(codes.InvalidArgument, "invalid parameter \"wut\""))
}

func TestCreateVolumeUnsupportedSource(t *testing.T) {
	testDirPath, _, cleanup := requireCluster(t)
	defer cleanup(t)

	req := makeCreateRequest(testDirPath, "foobar")
	req.VolumeContentSource = &csi.VolumeContentSource{
		Type: &csi.VolumeContentSource_Volume{
			Volume: &csi.VolumeContentSource_VolumeSource{
				VolumeId: "blah",
			},
		},
	}

	_, err := initTestController(t).CreateVolume(context.TODO(), &req)

	assert.Equal(t, err, status.Error(codes.InvalidArgument, "Volume source unsupported"))
}

func TestCreateVolumeMissingSecrets(t *testing.T) {
	testDirPath, _, cleanup := requireCluster(t)
	defer cleanup(t)

	req := makeCreateRequest(testDirPath, "foobar")
	req.Secrets = map[string]string{}

	_, err := initTestController(t).CreateVolume(context.TODO(), &req)

	assert.Equal(
		t,
		err,
		status.Error(codes.Unauthenticated, "username and password secrets missing",
	))
}

func TestCreateVolumeAuthFailure(t *testing.T) {
	testDirPath, _, cleanup := requireCluster(t)
	defer cleanup(t)

	req := makeCreateRequest(testDirPath, "foobar")
	req.Secrets["password"] = testPassword + "asdf"

	_, err := initTestController(t).CreateVolume(context.TODO(), &req)
	assert.Equal(t, err, status.Error(codes.Unauthenticated, "Login failed: 401"))
}

func TestCreateVolumeParentDirectoryMissing(t *testing.T) {
	testDirPath, _, cleanup := requireCluster(t)
	defer cleanup(t)

	req := makeCreateRequest(testDirPath, "foobar")
	badDir := testDirPath + "XYZ"
	req.Parameters[paramStoreRealPath] = badDir

	_, err := initTestController(t).CreateVolume(context.TODO(), &req)
	expectedMsg := fmt.Sprintf(
		"storerealpath directory %q missing for volume %q",
		badDir,
		makeVolumeId(badDir, "foobar"),
	)
	assert.Equal(t, err, status.Error(codes.NotFound, expectedMsg))
}

func TestCreateVolumeExistingFile(t *testing.T) {
	testDirPath, _, cleanup := requireCluster(t)
	defer cleanup(t)

	volumeDir := "confict"

	_, err := testConnection.CreateFile(testDirPath, volumeDir)
	assert.NoError(t, err)

	req := makeCreateRequest(testDirPath, volumeDir)

	_, err = initTestController(t).CreateVolume(context.TODO(), &req)
	expectedMsg := fmt.Sprintf(
		"A non-directory entity exists at %q for volume %q",
		testDirPath+"/"+volumeDir,
		makeVolumeId(testDirPath, volumeDir),
	)
	assert.Equal(t, err, status.Error(codes.AlreadyExists, expectedMsg))
}

func TestCreateVolumeHappyPath(t *testing.T) {
	testDirPath, _, cleanup := requireCluster(t)
	defer cleanup(t)

	volumeDir := "vol1"
	req := makeCreateRequest(testDirPath, volumeDir)

	resp, err := initTestController(t).CreateVolume(context.TODO(), &req)

	assert.NoError(t, err)
	assert.Equal(t, resp, makeCreateResponse(testDirPath, volumeDir))

	qVol, err := makeQumuloVolumeFromID(resp.Volume.VolumeId)
	assert.NoError(t, err)
	attributes, err := testConnection.LookUp(qVol.getVolumeRealPath())
	assert.NoError(t, err)
	assert.Equal(t, attributes.Mode, "0777")

	quotaLimit, err := testConnection.GetQuota(attributes.Id)
	assert.NoError(t, err)
	assert.Equal(t, quotaLimit, uint64(1024*1024*1024))
}

func TestCreateVolumeDirectoryAndQuotaExists(t *testing.T) {
	testDirPath, _, cleanup := requireCluster(t)
	defer cleanup(t)

	volumeDir := "vol1"
	attributes, err := testConnection.CreateDir(testDirPath, volumeDir)
	assert.NoError(t, err)
	err = testConnection.CreateQuota(attributes.Id, 2*1024*1024*1024)
	assert.NoError(t, err)
	attributes, err = testConnection.FileChmod(attributes.Id, "0555")
	assert.NoError(t, err)

	req := makeCreateRequest(testDirPath, volumeDir)
	resp, err := initTestController(t).CreateVolume(context.TODO(), &req)

	assert.NoError(t, err)
	assert.Equal(t, resp, makeCreateResponse(testDirPath, volumeDir))

	qVol, err := makeQumuloVolumeFromID(resp.Volume.VolumeId)
	assert.NoError(t, err)
	attributes, err = testConnection.LookUp(qVol.getVolumeRealPath())
	assert.NoError(t, err)
	assert.Equal(t, attributes.Mode, "0777")

	quotaLimit, err := testConnection.GetQuota(attributes.Id)
	assert.NoError(t, err)
	assert.Equal(t, quotaLimit, uint64(1024*1024*1024))
}

/*  _____                            ___     __    _
 * | ____|_  ___ __   __ _ _ __   __| \ \   / /__ | |_   _ _ __ ___   ___
 * |  _| \ \/ / '_ \ / _` | '_ \ / _` |\ \ / / _ \| | | | | '_ ` _ \ / _ \
 * | |___ >  <| |_) | (_| | | | | (_| | \ V / (_) | | |_| | | | | | |  __/
 * |_____/_/\_\ .__/ \__,_|_| |_|\__,_|  \_/ \___/|_|\__,_|_| |_| |_|\___|
 *            |_|
 *  FIGLET: ExpandVolume
 */

func TestExpandVolumeVolumeIdMissing(t *testing.T) {
	cs := initTestController(t)

	req := &csi.ControllerExpandVolumeRequest{}

	_, err := cs.ControllerExpandVolume(context.TODO(), req)
	assert.Equal(t, err, status.Error(codes.InvalidArgument, "Volume ID missing in request"))
}

func TestExpandVolumeVolumeInvalidId(t *testing.T) {
	cs := initTestController(t)

	req := &csi.ControllerExpandVolumeRequest{VolumeId: "blah-blah"}

	_, err := cs.ControllerExpandVolume(context.TODO(), req)
	assert.Equal(t, err, status.Error(codes.NotFound, "Volume not found \"blah-blah\""))
}

func TestExpandVolumeVolumeNoLimit(t *testing.T) {
	cs := initTestController(t)

	volumeId := "v1:server:123//////foobar"

	req := &csi.ControllerExpandVolumeRequest{VolumeId: volumeId}

	_, err := cs.ControllerExpandVolume(context.TODO(), req)
	assert.Equal(t, err, status.Error(codes.InvalidArgument, "CapacityRange must be provided"))
}

func TestExpandVolumeMissingSecrets(t *testing.T) {
	testDirPath, _, cleanup := requireCluster(t)
	defer cleanup(t)

	cs := initTestController(t)

	volumeId := makeVolumeId(testDirPath, "foobar")

	req := &csi.ControllerExpandVolumeRequest{
		VolumeId:      volumeId,
		CapacityRange: &csi.CapacityRange{RequiredBytes: 1024 * 1024 * 1024},
		Secrets:       map[string]string{},
	}

	_, err := cs.ControllerExpandVolume(context.TODO(), req)
	assert.Equal(
		t,
		err,
		status.Error(codes.Unauthenticated, "username and password secrets missing",
	))
}

func TestExpandVolumeAuthFailure(t *testing.T) {
	testDirPath, _, cleanup := requireCluster(t)
	defer cleanup(t)

	cs := initTestController(t)

	volumeId := makeVolumeId(testDirPath, "foobar")

	req := &csi.ControllerExpandVolumeRequest{
		VolumeId:      volumeId,
		CapacityRange: &csi.CapacityRange{RequiredBytes: 1024 * 1024 * 1024},
		Secrets: map[string]string{
			"username": testUsername,
			"password": testPassword + "asdf",
		},
	}

	_, err := cs.ControllerExpandVolume(context.TODO(), req)
	assert.Equal(t, err, status.Error(codes.Unauthenticated, "Login failed: 401"))
}

func TestExpandVolumeVolumeDirectoryNotFound(t *testing.T) {
	testDirPath, _, cleanup := requireCluster(t)
	defer cleanup(t)

	cs := initTestController(t)

	volumeId := makeVolumeId(testDirPath, "foobar")

	req := &csi.ControllerExpandVolumeRequest{
		VolumeId:      volumeId,
		CapacityRange: &csi.CapacityRange{RequiredBytes: 1024 * 1024 * 1024},
		Secrets: map[string]string{
			"username": testUsername,
			"password": testPassword,
		},
	}

	_, err := cs.ControllerExpandVolume(context.TODO(), req)
	assert.Equal(t, err, status.Errorf(codes.NotFound, "Directory for volume %q is missing", volumeId))
}

func TestExpandVolumeVolumeHappyPath(t *testing.T) {
	testDirPath, _, cleanup := requireCluster(t)
	defer cleanup(t)

	cs := initTestController(t)

	volumeId := makeVolumeId(testDirPath, "foobar")

	req := &csi.ControllerExpandVolumeRequest{
		VolumeId:      volumeId,
		CapacityRange: &csi.CapacityRange{RequiredBytes: 2 * 1024 * 1024 * 1024},
		Secrets: map[string]string{
			"username": testUsername,
			"password": testPassword,
		},
	}

	// Create dir and quota before operation.
	attributes, err := testConnection.EnsureDir(testDirPath, "foobar")
	assert.NoError(t, err)
	err = testConnection.EnsureQuota(attributes.Id, 1024*1024*1024)

	resp, err := cs.ControllerExpandVolume(context.TODO(), req)
	assert.NoError(t, err)
	assert.Equal(t, resp, &csi.ControllerExpandVolumeResponse{
		CapacityBytes:         2 * 1024 * 1024 * 1024,
		NodeExpansionRequired: false,
	})

	newLimit, err := testConnection.GetQuota(attributes.Id)
	assert.NoError(t, err)
	assert.Equal(t, newLimit, uint64(2*1024*1024*1024))
}

/*  ____       _      _     __     __    _
 * |  _ \  ___| | ___| |_ __\ \   / /__ | |_   _ _ __ ___   ___
 * | | | |/ _ \ |/ _ \ __/ _ \ \ / / _ \| | | | | '_ ` _ \ / _ \
 * | |_| |  __/ |  __/ ||  __/\ V / (_) | | |_| | | | | | |  __/
 * |____/ \___|_|\___|\__\___| \_/ \___/|_|\__,_|_| |_| |_|\___|
 *  FIGLET: DeleteVolume
 */

func TestDeleteVolumeVolumeIdMissing(t *testing.T) {
	cs := initTestController(t)

	req := &csi.DeleteVolumeRequest{}

	_, err := cs.DeleteVolume(context.TODO(), req)
	assert.Equal(t, err, status.Error(codes.InvalidArgument, "Volume ID missing in request"))
}

func TestDeleteVolumeInvalidVolumeId(t *testing.T) {
	cs := initTestController(t)

	req := &csi.DeleteVolumeRequest{VolumeId: "invalid ignore per code"}
	_, err := makeQumuloVolumeFromID(req.VolumeId)
	assert.Error(t, err)

	resp, err := cs.DeleteVolume(context.TODO(), req)

	assert.NoError(t, err)
	assert.Equal(t, resp, &csi.DeleteVolumeResponse{})
}

func TestDeleteVolumeMissingSecrets(t *testing.T) {
	testDirPath, _, cleanup := requireCluster(t)
	defer cleanup(t)

	cs := initTestController(t)

	volumeId := makeVolumeId(testDirPath, "foobar")

	req := &csi.DeleteVolumeRequest{
		VolumeId: volumeId,
		Secrets:  map[string]string{},
	}

	_, err := cs.DeleteVolume(context.TODO(), req)
	assert.Equal(t, err, status.Error(codes.Unauthenticated, "username and password secrets missing"))
}

func TestDeleteVolumeAuthFailure(t *testing.T) {
	testDirPath, _, cleanup := requireCluster(t)
	defer cleanup(t)

	cs := initTestController(t)

	volumeId := makeVolumeId(testDirPath, "foobar")

	req := &csi.DeleteVolumeRequest{
		VolumeId: volumeId,
		Secrets: map[string]string{
			"username": testUsername,
			"password": testPassword + "asdf",
		},
	}

	_, err := cs.DeleteVolume(context.TODO(), req)
	assert.Equal(t, err, status.Error(codes.Unauthenticated, "Login failed: 401"))
}

func TestDeleteVolumeHappyPath(t *testing.T) {
	testDirPath, _, cleanup := requireCluster(t)
	defer cleanup(t)

	cs := initTestController(t)

	volumeId := makeVolumeId(testDirPath, "foobar")

	req := &csi.DeleteVolumeRequest{
		VolumeId: volumeId,
		Secrets: map[string]string{
			"username": testUsername,
			"password": testPassword,
		},
	}

	// Create dir and test lookup before operation.
	qVol, err := makeQumuloVolumeFromID(req.VolumeId)
	assert.NoError(t, err)
	_, err = testConnection.EnsureDir(testDirPath, "foobar")
	assert.NoError(t, err)
	_, err = testConnection.LookUp(qVol.getVolumeRealPath())
	assert.NoError(t, err)

	// Run
	resp, err := cs.DeleteVolume(context.TODO(), req)
	assert.NoError(t, err)
	assert.Equal(t, resp, &csi.DeleteVolumeResponse{})

	// Directory deleted
	_, err = testConnection.LookUp(qVol.getVolumeRealPath())
	assert.True(t, errorIsRestErrorWithStatus(err, 404))
}

func TestDeleteVolumeMissingDirectory(t *testing.T) {
	testDirPath, _, cleanup := requireCluster(t)
	defer cleanup(t)

	cs := initTestController(t)

	volumeId := makeVolumeId(testDirPath, "foobar")

	req := &csi.DeleteVolumeRequest{
		VolumeId: volumeId,
		Secrets: map[string]string{
			"username": testUsername,
			"password": testPassword,
		},
	}

	// No directory exists, should still be success.
	qVol, err := makeQumuloVolumeFromID(req.VolumeId)
	assert.NoError(t, err)
	_, err = testConnection.LookUp(qVol.getVolumeRealPath())
	assert.True(t, errorIsRestErrorWithStatus(err, 404))

	// Run
	resp, err := cs.DeleteVolume(context.TODO(), req)
	assert.NoError(t, err)
	assert.Equal(t, resp, &csi.DeleteVolumeResponse{})
}

/* __     __    _ _     _       _     __     __    _
 * \ \   / /_ _| (_) __| | __ _| |_ __\ \   / /__ | |_   _ _ __ ___   ___
 *  \ \ / / _` | | |/ _` |/ _` | __/ _ \ \ / / _ \| | | | | '_ ` _ \ / _ \
 *   \ V / (_| | | | (_| | (_| | ||  __/\ V / (_) | | |_| | | | | | |  __/
 *    \_/ \__,_|_|_|\__,_|\__,_|\__\___| \_/ \___/|_|\__,_|_| |_| |_|\___|
 *   ____                  _     _ _ _ _   _
 *  / ___|__ _ _ __   __ _| |__ (_) (_) |_(_) ___  ___
 * | |   / _` | '_ \ / _` | '_ \| | | | __| |/ _ \/ __|
 * | |__| (_| | |_) | (_| | |_) | | | | |_| |  __/\__ \
 *  \____\__,_| .__/ \__,_|_.__/|_|_|_|\__|_|\___||___/
 *            |_|
 *  FIGLET: ValidateVolumeCapabilities
 */

func TestValidateVolumeCapabilities(t *testing.T) {
	cases := []struct {
		desc        string
		req         *csi.ValidateVolumeCapabilitiesRequest
		resp        *csi.ValidateVolumeCapabilitiesResponse
		expectedErr error
	}{
		{
			desc:        "Volume ID missing",
			req:         &csi.ValidateVolumeCapabilitiesRequest{},
			resp:        nil,
			expectedErr: status.Error(codes.InvalidArgument, "Volume ID missing in request"),
		},
		{
			desc:        "Volume capabilities missing",
			req:         &csi.ValidateVolumeCapabilitiesRequest{VolumeId: testVolumeID},
			resp:        nil,
			expectedErr: status.Error(codes.InvalidArgument, "Volume capabilities missing in request"),
		},
		{
			desc: "valid request",
			req: &csi.ValidateVolumeCapabilitiesRequest{
				VolumeId: testVolumeID,
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
						},
					},
				},
			},
			resp:        &csi.ValidateVolumeCapabilitiesResponse{Message: ""},
			expectedErr: nil,
		},
	}

	for _, test := range cases {
		test := test //pin
		t.Run(test.desc, func(t *testing.T) {
			// Setup
			cs := initTestController(t)

			// Run
			resp, err := cs.ValidateVolumeCapabilities(context.TODO(), test.req)

			// Verify
			if test.expectedErr == nil && err != nil {
				t.Errorf("test %q failed: %v", test.desc, err)
			}
			if test.expectedErr != nil && err == nil {
				t.Errorf("test %q failed; expected error %v, got success", test.desc, test.expectedErr)
			}
			if !reflect.DeepEqual(resp, test.resp) {
				t.Errorf("test %q failed: got resp %+v, expected %+v", test.desc, resp, test.resp)
			}
		})
	}
}

/*   ____            _             _ _
 *  / ___|___  _ __ | |_ _ __ ___ | | | ___ _ __
 * | |   / _ \| '_ \| __| '__/ _ \| | |/ _ \ '__|
 * | |__| (_) | | | | |_| | | (_) | | |  __/ |
 *  \____\___/|_| |_|\__|_|  \___/|_|_|\___|_|
 *   ____      _    ____                  _     _ _ _ _
 *  / ___| ___| |_ / ___|__ _ _ __   __ _| |__ (_) (_) |_ ___  ___
 * | |  _ / _ \ __| |   / _` | '_ \ / _` | '_ \| | | | __/ _ \/ __|
 * | |_| |  __/ |_| |__| (_| | |_) | (_| | |_) | | | | ||  __/\__ \
 *  \____|\___|\__|\____\__,_| .__/ \__,_|_.__/|_|_|_|\__\___||___/
 *                           |_|
 *  FIGLET: ControllerGetCapabilites
 */

func TestControllerGetCapabilities(t *testing.T) {
	req := &csi.ControllerGetCapabilitiesRequest{}
	expectedResp := &csi.ControllerGetCapabilitiesResponse{
		Capabilities: []*csi.ControllerServiceCapability{
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
					},
				},
			},
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_SINGLE_NODE_MULTI_WRITER,
					},
				},
			},
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
					},
				},
			},
		},
	}

	// Setup
	cs := initTestController(t)

	// Run
	resp, err := cs.ControllerGetCapabilities(context.TODO(), req)

	// Verify
	assert.NoError(t, err)
	if !reflect.DeepEqual(resp, expectedResp) {
		t.Errorf("got resp %+v, expected %+v", resp, expectedResp)
	}
}

/*  _   _      _
 * | | | | ___| |_ __   ___ _ __ ___
 * | |_| |/ _ \ | '_ \ / _ \ '__/ __|
 * |  _  |  __/ | |_) |  __/ |  \__ \
 * |_| |_|\___|_| .__/ \___|_|  |___/
 *              |_|
 *  FIGLET: Helpers
 */

func TestGetQumuloVolumeFromID(t *testing.T) {
	cases := []struct {
		name      string
		req       string
		expectRet *qumuloVolume
		expectErr string
	}{
		{
			name:      "ID only server",
			req:       "blah blah",
			expectRet: nil,
			expectErr: "Could not decode volume ID \"blah blah\"",
		},
		{
			name:      "Unknown version",
			req:       "v3:server1:444//////volume",
			expectRet: nil,
			expectErr: "Could not decode volume ID \"v3:server1:444//////volume\"",
		},
		{
			name:      "Bad port",
			req:       "v1:server1:4foo//////volume",
			expectRet: nil,
			expectErr: "Could not decode volume ID \"v1:server1:4foo//////volume\"",
		},
		{
			name: "Happy store path root, mount path root",
			req:  "v1:server1:444//////volume",
			expectRet: &qumuloVolume{
				id:             "v1:server1:444//////volume",
				server:         "server1",
				restPort:       444,
				storeRealPath:  "",
				storeMountPath: "",
				name:           "volume",
			},
			expectErr: "",
		},
		{
			name: "Happy store path non-root, mount path root",
			req:  "v1:server1:444//foo/bar/baz////volume",
			expectRet: &qumuloVolume{
				id:             "v1:server1:444//foo/bar/baz////volume",
				server:         "server1",
				restPort:       444,
				storeRealPath:  "foo/bar/baz",
				storeMountPath: "",
				name:           "volume",
			},
			expectErr: "",
		},
		{
			name: "Happy store path non-root, mount path non-root",
			req:  "v1:server1:444//foo/bar/baz//some/export//frog",
			expectRet: &qumuloVolume{
				id:             "v1:server1:444//foo/bar/baz//some/export//frog",
				server:         "server1",
				restPort:       444,
				storeRealPath:  "foo/bar/baz",
				storeMountPath: "some/export",
				name:           "frog",
			},
			expectErr: "",
		},
	}

	for _, test := range cases {
		test := test //pin
		t.Run(test.name, func(t *testing.T) {
			// Run
			ret, err := makeQumuloVolumeFromID(test.req)

			// Verify
			if len(test.expectErr) != 0 {
				assert.EqualError(t, err, test.expectErr)
			} else {
				assert.NoError(t, err)
				if !reflect.DeepEqual(ret, test.expectRet) {
					t.Errorf("test %q failed: %+v != %+v", test.name, ret, test.expectRet)
				}
			}
		})
	}
}

func TestGetQuotaLimit(t *testing.T) {
	cases := []struct {
		name      string
		input     *csi.CapacityRange
		expectErr string
		expectRet uint64
	}{
		{
			name:      "nil input",
			input:     nil,
			expectErr: "rpc error: code = InvalidArgument desc = CapacityRange must be provided",
			expectRet: 0,
		},
		{
			name:      "both zero",
			input:     &csi.CapacityRange{RequiredBytes: 0, LimitBytes: 0},
			expectErr: "rpc error: code = InvalidArgument desc = RequiredBytes or LimitBytes must be provided",
			expectRet: 0,
		},
		{
			name:      "required negative, limit zero",
			input:     &csi.CapacityRange{RequiredBytes: -1, LimitBytes: 0},
			expectErr: "rpc error: code = InvalidArgument desc = RequiredBytes must be positive",
			expectRet: 0,
		},
		{
			name:      "required zero, limit negative",
			input:     &csi.CapacityRange{RequiredBytes: 0, LimitBytes: -1},
			expectErr: "rpc error: code = InvalidArgument desc = LimitBytes must be positive",
			expectRet: 0,
		},
		{
			name:      "required used first",
			input:     &csi.CapacityRange{RequiredBytes: 100, LimitBytes: 50},
			expectErr: "",
			expectRet: 100,
		},
		{
			name:      "limit used if required is zero",
			input:     &csi.CapacityRange{RequiredBytes: 0, LimitBytes: 50},
			expectErr: "",
			expectRet: 50,
		},
	}

	for _, test := range cases {
		test := test //pin
		t.Run(test.name, func(t *testing.T) {
			// Setup

			// Run
			limit, err := getQuotaLimit(test.input)

			// Verify
			if len(test.expectErr) > 0 {
				if err == nil {
					t.Errorf("test %q failed; expected err", test.name)
				}

				if err.Error() != test.expectErr {
					t.Errorf("test %q failed; expected err %v != %v ", test.name, err.Error(), test.expectErr)
				}
			} else {
				if err != nil {
					t.Errorf("test %q failed; expected nil err: %v", test.name, err)
				}
				if limit != test.expectRet {
					t.Errorf("test %q failed; limit %d != %d", test.name, limit, test.expectRet)
				}
			}
		})
	}
}

func TestNewQumuloVolume(t *testing.T) {
	cases := []struct {
		name      string
		volName   string
		params    map[string]string
		expectErr error
		expectVol *qumuloVolume
	}{
		{
			name:    "non-numeric port",
			volName: "vol1",
			params: map[string]string{
				"restport": "x y",
			},
			expectErr: status.Error(codes.InvalidArgument, "invalid port \"x y\""),
			expectVol: nil,
		},
		{
			name:    "unknown parameter",
			volName: "vol1",
			params: map[string]string{
				"blah": "blah",
			},
			expectErr: status.Error(codes.InvalidArgument, "invalid parameter \"blah\""),
			expectVol: nil,
		},
		{
			name:    "server is required parameter",
			volName: "vol1",
			params: map[string]string{
				"StoreRealPath": "/foo",
			},
			expectErr: status.Error(codes.InvalidArgument, "server is a required parameter"),
			expectVol: nil,
		},
		{
			name:    "storerealpath is required parameter",
			volName: "vol1",
			params: map[string]string{
				"server": "foo",
			},
			expectErr: status.Error(codes.InvalidArgument, "storerealpath is a required parameter"),
			expectVol: nil,
		},
		{
			name:    "storerealpath must start with slash",
			volName: "vol1",
			params: map[string]string{
				"server":        "foo",
				"StoreRealPath": "foo",
			},
			expectErr: status.Error(
				codes.InvalidArgument,
				"storerealpath (\"foo\") must start with a '/'",
			),
			expectVol: nil,
		},
		{
			name:    "storemountpath must start with slash",
			volName: "vol1",
			params: map[string]string{
				"server":         "foo",
				"StoreRealPath":  "/foo",
				"storemountpath": "blah",
			},
			expectErr: status.Error(
				codes.InvalidArgument,
				"storemountpath (\"blah\") must start with a '/'",
			),
			expectVol: nil,
		},
		{
			name:    "default path and mount",
			volName: "vol1",
			params: map[string]string{
				"server":        "somserver",
				"StoreRealPath": "/foo/bar",
			},
			expectErr: nil,
			expectVol: &qumuloVolume{
				id:             "v1:somserver:8000//foo/bar//foo/bar//vol1",
				server:         "somserver",
				restPort:       8000,
				storeRealPath:  "foo/bar",
				storeMountPath: "foo/bar",
				name:           "vol1",
			},
		},
		{
			name:    "custom port",
			volName: "vol1",
			params: map[string]string{
				"server":        "somserver",
				"StoreRealPath": "/foo/bar",
				"restport":      "1234",
			},
			expectErr: nil,
			expectVol: &qumuloVolume{
				id:             "v1:somserver:1234//foo/bar//foo/bar//vol1",
				server:         "somserver",
				restPort:       1234,
				storeRealPath:  "foo/bar",
				storeMountPath: "foo/bar",
				name:           "vol1",
			},
		},
		{
			name:    "root path and default mount",
			volName: "vol1",
			params: map[string]string{
				"server":        "somserver",
				"StoreRealPath": "/",
			},
			expectErr: nil,
			expectVol: &qumuloVolume{
				id:             "v1:somserver:8000//////vol1",
				server:         "somserver",
				restPort:       8000,
				storeRealPath:  "",
				storeMountPath: "",
				name:           "vol1",
			},
		},
		{
			name:    "root path and mount",
			volName: "vol1",
			params: map[string]string{
				"server":         "somserver",
				"StoreRealPath":  "/",
				"storeMountPath": "/",
			},
			expectErr: nil,
			expectVol: &qumuloVolume{
				id:             "v1:somserver:8000//////vol1",
				server:         "somserver",
				restPort:       8000,
				storeRealPath:  "",
				storeMountPath: "",
				name:           "vol1",
			},
		},
		{
			name:    "path and mount",
			volName: "vol1",
			params: map[string]string{
				"server":         "somserver",
				"StoreRealPath":  "/x/y",
				"storeMountPath": "/y/z",
			},
			expectErr: nil,
			expectVol: &qumuloVolume{
				id:             "v1:somserver:8000//x/y//y/z//vol1",
				server:         "somserver",
				restPort:       8000,
				storeRealPath:  "x/y",
				storeMountPath: "y/z",
				name:           "vol1",
			},
		},
		{
			name:    "extra leading and trailing slashes",
			volName: "vol1",
			params: map[string]string{
				"server":         "somserver",
				"StoreRealPath":  "///x/y/",
				"storeMountPath": "//y/z///",
			},
			expectErr: nil,
			expectVol: &qumuloVolume{
				id:             "v1:somserver:8000//x/y//y/z//vol1",
				server:         "somserver",
				restPort:       8000,
				storeRealPath:  "x/y",
				storeMountPath: "y/z",
				name:           "vol1",
			},
		},
		{
			name:    "extra interior slashes",
			volName: "vol1",
			params: map[string]string{
				"server":         "somserver",
				"StoreRealPath":  "/a//b///c",
				"storeMountPath": "/d//e///f",
			},
			expectErr: nil,
			expectVol: &qumuloVolume{
				id:             "v1:somserver:8000//a/b/c//d/e/f//vol1",
				server:         "somserver",
				restPort:       8000,
				storeRealPath:  "a/b/c",
				storeMountPath: "d/e/f",
				name:           "vol1",
			},
		},
	}

	for _, test := range cases {
		test := test //pin
		t.Run(test.name, func(t *testing.T) {
			assert.NotEqual(t, test.expectErr == nil, test.expectVol == nil)

			// Setup

			// Run
			vol, err := newQumuloVolume(test.volName, test.params)

			if test.expectErr != nil {
				assert.Nil(t, vol)
				assert.Equal(t, err, test.expectErr)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, vol, test.expectVol)
			}
		})
	}
}

func TestGetVolumeRealPathEmpty(t *testing.T) {
	vol := qumuloVolume{
		id:             "v1:somserver:8000////d/e/f//vol1",
		server:         "somserver",
		restPort:       8000,
		storeRealPath:  "",
		storeMountPath: "d/e/f",
		name:           "vol1",
	}

	assert.Equal(t, vol.getVolumeRealPath(), "/vol1")
}

func TestGetVolumeRealPathNonEmpty(t *testing.T) {
	vol := qumuloVolume{
		id:             "v1:somserver:8000////d/e/f//vol1",
		server:         "somserver",
		restPort:       8000,
		storeRealPath:  "hello/world",
		storeMountPath: "d/e/f",
		name:           "vol1",
	}

	assert.Equal(t, vol.getVolumeRealPath(), "/hello/world/vol1")
}

func TestGetVolumeSharePathEmpty(t *testing.T) {
	vol := qumuloVolume{
		id:             "v1:somserver:8000////d/e/f//vol1",
		server:         "somserver",
		restPort:       8000,
		storeRealPath:  "d/e/f",
		storeMountPath: "",
		name:           "vol1",
	}

	assert.Equal(t, vol.getVolumeSharePath(), "/vol1")
}

func TestGetVolumeShareNonPathEmpty(t *testing.T) {
	vol := qumuloVolume{
		id:             "v1:somserver:8000////d/e/f//vol1",
		server:         "somserver",
		restPort:       8000,
		storeRealPath:  "d/e/f",
		storeMountPath: "x/y/z",
		name:           "vol1",
	}

	assert.Equal(t, vol.getVolumeSharePath(), "/x/y/z/vol1")
}

func TestQumuloVolumeToCSIVolume(t *testing.T) {
	vol := qumuloVolume{
		id:             "v1:somserver:8000////d/e/f//vol1",
		server:         "somserver",
		restPort:       8000,
		storeRealPath:  "d/e/f",
		storeMountPath: "x/y/z",
		name:           "vol1",
	}

	assert.Equal(
		t,
		vol.qumuloVolumeToCSIVolume(),
		&csi.Volume{
			CapacityBytes: 0,
			VolumeId:      "v1:somserver:8000////d/e/f//vol1",
			VolumeContext: map[string]string{
				paramServer: "somserver",
				paramShare:  "/x/y/z/vol1",
			},
		},
	)
}
