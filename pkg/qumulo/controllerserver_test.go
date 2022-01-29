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
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"fmt"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	mount "k8s.io/mount-utils"
)

const (
	testCSIVolume      = "test-csi"
	testVolumeID       = "test-server/test-base-dir/test-csi"
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

func TestCreateVolume(t *testing.T) {
	testDirPath, _, cleanup := requireCluster(t)
	defer cleanup(t)

	volumeId := fmt.Sprintf("v1:%s:%d//%s////foobar", testHost, testPort, strings.Trim(testDirPath, "/"))

	t.Logf("TEST PORT is %d", testPort)

	happyRequest := csi.CreateVolumeRequest{
		Name: "foobar",
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

	type MakeRequest func() *csi.CreateVolumeRequest

	cases := []struct {
		name      string
		makeReq   MakeRequest
		expectRet *csi.CreateVolumeResponse
		expectErr string
	}{
		{
			name: "happy path",
			makeReq: func() *csi.CreateVolumeRequest { return &happyRequest },
			expectRet: &csi.CreateVolumeResponse{
				Volume: &csi.Volume{
					VolumeId: volumeId,
					VolumeContext: map[string]string{
						paramServer: testHost,
						paramShare:  "/foobar",
					},
				},
			},
		},
		{
			name: "name empty",
			makeReq: func() *csi.CreateVolumeRequest {
				r := happyRequest
				r.Name = ""
				return &r
			},
			expectErr: "rpc error: code = InvalidArgument desc = CreateVolume name must be provided",
		},
		{
			name: "invalid volume capabilities",
			makeReq: func() *csi.CreateVolumeRequest {
				r := happyRequest
				r.VolumeCapabilities = []*csi.VolumeCapability{
					{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
						},
					},
				}
				return &r
			},
			expectErr: "rpc error: code = InvalidArgument desc = volume capability access type not set",
		},
		{
			name: "unknown parameter",
			makeReq: func() *csi.CreateVolumeRequest {
				r := happyRequest
				r.Parameters["wut"] = "ever"
				return &r
			},
			expectErr: "rpc error: code = InvalidArgument desc = invalid parameter \"wut\"",
		},
	}

	// XXX scott:
	// * can test the directory and quota exist
	// * will probably have cross talk of tests
	// * so maybe separate tests
	// * test idempotency
	// * more cases and errors

	for _, test := range cases {
		test := test //pin
		t.Run(test.name, func(t *testing.T) {
			// Setup
			cs := initTestController(t)

			// Run
			ret, err := cs.CreateVolume(context.TODO(), test.makeReq())

			// Verify
			t.Log(ret)
			if len(test.expectErr) != 0 {
				assert.EqualError(t, err, test.expectErr)
			} else {
				assert.NoError(t, err)
				if !reflect.DeepEqual(ret, test.expectRet) {
					t.Errorf("test %q failed: got %+v, expected %+v", test.name, ret, test.expectRet)
				}
			}
		})
	}
}

// XXX scott: expand volume

func TestDeleteVolume(t *testing.T) {
	// XXX scott: all of this

	cases := []struct {
		desc        string
		req         *csi.DeleteVolumeRequest
		resp        *csi.DeleteVolumeResponse
		expectedErr error
	}{
		{
			desc:        "Volume ID missing",
			req:         &csi.DeleteVolumeRequest{},
			resp:        nil,
			expectedErr: status.Error(codes.InvalidArgument, "Volume ID missing in request"),
		},
		{
			desc:        "Valid request",
			req:         &csi.DeleteVolumeRequest{VolumeId: testVolumeID},
			resp:        &csi.DeleteVolumeResponse{},
			expectedErr: nil,
		},
	}

	for _, test := range cases {
		test := test //pin
		workingMountDir := "/tmp" // XXX scott
		t.Run(test.desc, func(t *testing.T) {
			// Setup
			cs := initTestController(t)
			_ = os.MkdirAll(filepath.Join(workingMountDir, testCSIVolume), os.ModePerm)
			_, _ = os.Create(filepath.Join(workingMountDir, testCSIVolume, testCSIVolume))

			// Run
			resp, err := cs.DeleteVolume(context.TODO(), test.req)

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
			if _, err := os.Stat(filepath.Join(workingMountDir, testCSIVolume, testCSIVolume)); test.expectedErr == nil && !os.IsNotExist(err) {
				t.Errorf("test %q failed: expected volume subdirectory deleted, it still exists", test.desc)
			}
		})
	}
}

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
			name:      "Happy store path root, mount path root",
			req:       "v1:server1:444//////volume",
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
			name:      "Happy store path non-root, mount path root",
			req:       "v1:server1:444//foo/bar/baz////volume",
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
			name:      "Happy store path non-root, mount path non-root",
			req:       "v1:server1:444//foo/bar/baz//some/export//frog",
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

	// XXX scott: we don't really need a controller for this function
	cs := initTestController(t)

	for _, test := range cases {
		test := test //pin
		t.Run(test.name, func(t *testing.T) {
			// Run
			ret, err := cs.getQumuloVolumeFromID(test.req)

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
			name:	   "nil input",
			input:     nil,
			expectErr: "rpc error: code = InvalidArgument desc = CapacityRange must be provided",
			expectRet: 0,
		},
		{
			name:	   "both zero",
			input:     &csi.CapacityRange{RequiredBytes: 0, LimitBytes: 0},
			expectErr: "rpc error: code = InvalidArgument desc = RequiredBytes or LimitBytes must be provided",
			expectRet: 0,
		},
		{
			name:	   "required negative, limit zero",
			input:     &csi.CapacityRange{RequiredBytes: -1, LimitBytes: 0},
			expectErr: "rpc error: code = InvalidArgument desc = RequiredBytes must be positive",
			expectRet: 0,
		},
		{
			name:	   "required zero, limit negative",
			input:     &csi.CapacityRange{RequiredBytes: 0, LimitBytes: -1},
			expectErr: "rpc error: code = InvalidArgument desc = LimitBytes must be positive",
			expectRet: 0,
		},
		{
			name:	   "required used first",
			input:     &csi.CapacityRange{RequiredBytes: 100, LimitBytes: 50},
			expectErr: "",
			expectRet: 100,
		},
		{
			name:	   "limit used if required is zero",
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
