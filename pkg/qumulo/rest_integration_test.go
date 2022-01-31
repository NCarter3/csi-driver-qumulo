package qumulo

import (
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
)

/*  _            _
 * | |_ ___  ___| |_ ___
 * | __/ _ \/ __| __/ __|
 * | ||  __/\__ \ |_\__ \
 *  \__\___||___/\__|___/
 *  FIGLET: tests
 */

func TestRestCreateDir(t *testing.T) {
	testDirPath, _, cleanup := requireCluster(t)
	defer cleanup(t)

	attributes, err := testConnection.CreateDir(testDirPath, "bar")
	assert.NoError(t, err)
	if attributes.Type != "FS_FILE_TYPE_DIRECTORY" {
		t.Fatalf("unexpected attributes %v", attributes)
	}
}

func TestRestCreateDirTwiceErrors(t *testing.T) {
	testDirPath, _, cleanup := requireCluster(t)
	defer cleanup(t)

	_, err := testConnection.CreateDir(testDirPath, "bar")
	assert.NoError(t, err)

	_, err = testConnection.CreateDir(testDirPath, "bar")
	assertRestError(t, err, 409, "fs_entry_exists_error")
}

func TestRestEnsureDirNewDir(t *testing.T) {
	testDirPath, _, cleanup := requireCluster(t)
	defer cleanup(t)

	attributes, err := testConnection.EnsureDir(testDirPath, "somedir")
	assert.NoError(t, err)
	if attributes.Type != "FS_FILE_TYPE_DIRECTORY" {
		t.Fatalf("unexpected attributes %v", attributes)
	}
}

func TestRestEnsureDirAfterCreateDir(t *testing.T) {
	testDirPath, _, cleanup := requireCluster(t)
	defer cleanup(t)

	attributes1, err := testConnection.EnsureDir(testDirPath, "somedir")
	assert.NoError(t, err)

	attributes2, err := testConnection.EnsureDir(testDirPath, "blah")
	assert.NoError(t, err)

	if attributes1 != attributes1 {
		t.Fatalf("unexpected attributes mismatch %v != %v", attributes1, attributes2)
	}
}

func TestRestEnsureDirTwice(t *testing.T) {
	testDirPath, _, cleanup := requireCluster(t)
	defer cleanup(t)

	attributes1, err := testConnection.EnsureDir(testDirPath, "blah")
	assert.NoError(t, err)

	attributes2, err := testConnection.EnsureDir(testDirPath, "blah")
	assert.NoError(t, err)

	if attributes1 != attributes1 {
		t.Fatalf("unexpected attributes mismatch %v != %v", attributes1, attributes2)
	}
}

func TestRestCreateFile(t *testing.T) {
	testDirPath, _, cleanup := requireCluster(t)
	defer cleanup(t)

	attributes, err := testConnection.CreateFile(testDirPath, "notadir")
	assert.NoError(t, err)
	if attributes.Type != "FS_FILE_TYPE_FILE" {
		t.Fatalf("unexpected attributes %v", attributes)
	}
}

func TestRestEnsureDirWithFileConflict(t *testing.T) {
	testDirPath, _, cleanup := requireCluster(t)
	defer cleanup(t)

	_, err := testConnection.CreateFile(testDirPath, "x")
	assert.NoError(t, err)

	_, err = testConnection.EnsureDir(testDirPath, "x")
	assertRestError(t, err, 409, "fs_entry_exists_error")
}

func TestRestCreateQuota(t *testing.T) {
	_, testDirId, cleanup := requireCluster(t)
	defer cleanup(t)

	newLimit := uint64(1024 * 1024 * 1024)
	err := testConnection.CreateQuota(testDirId, newLimit)
	assert.NoError(t, err)

	limit, err := testConnection.GetQuota(testDirId)
	assert.NoError(t, err)
	assert.Equal(t, limit, newLimit)
}

func TestRestCreateQuotaTwiceErrors(t *testing.T) {
	_, testDirId, cleanup := requireCluster(t)
	defer cleanup(t)

	err := testConnection.CreateQuota(testDirId, 1024*1024*1024)
	assert.NoError(t, err)

	err = testConnection.CreateQuota(testDirId, 1024*1024*1024)
	assertRestError(t, err, 409, "api_quotas_quota_limit_already_set_error")
}

func TestRestUpdateQuotaNoQuotaErrors(t *testing.T) {
	_, testDirId, cleanup := requireCluster(t)
	defer cleanup(t)

	err := testConnection.UpdateQuota(testDirId, 1024*1024*1024)
	assertRestError(t, err, 404, "api_quotas_quota_limit_not_found_error")
}

func TestRestUpdateQuotaAfterCreateQuota(t *testing.T) {
	_, testDirId, cleanup := requireCluster(t)
	defer cleanup(t)

	newLimit := uint64(1024 * 1024 * 1024)
	err := testConnection.CreateQuota(testDirId, newLimit)
	assert.NoError(t, err)

	err = testConnection.UpdateQuota(testDirId, newLimit)
	assert.NoError(t, err)

	limit, err := testConnection.GetQuota(testDirId)
	assert.NoError(t, err)
	assert.Equal(t, limit, newLimit)
}

func TestRestEnsureQuotaNewQuota(t *testing.T) {
	_, testDirId, cleanup := requireCluster(t)
	defer cleanup(t)

	newLimit := uint64(1024 * 1024 * 1024)
	err := testConnection.EnsureQuota(testDirId, newLimit)
	assert.NoError(t, err)

	limit, err := testConnection.GetQuota(testDirId)
	assert.NoError(t, err)
	assert.Equal(t, limit, newLimit)
}

func TestRestEnsureQuotaAfterCreateQuota(t *testing.T) {
	_, testDirId, cleanup := requireCluster(t)
	defer cleanup(t)

	err := testConnection.CreateQuota(testDirId, 1024*1024*1024)
	assert.NoError(t, err)

	newLimit := uint64(2 * 1024 * 1024 * 1024)
	err = testConnection.EnsureQuota(testDirId, newLimit)
	assert.NoError(t, err)

	limit, err := testConnection.GetQuota(testDirId)
	assert.NoError(t, err)
	assert.Equal(t, limit, newLimit)
}

func TestRestEnsureQuotaTwice(t *testing.T) {
	_, testDirId, cleanup := requireCluster(t)
	defer cleanup(t)

	newLimit := uint64(2 * 1024 * 1024 * 1024)
	err := testConnection.EnsureQuota(testDirId, newLimit)
	assert.NoError(t, err)

	err = testConnection.EnsureQuota(testDirId, newLimit)
	assert.NoError(t, err)

	limit, err := testConnection.GetQuota(testDirId)
	assert.NoError(t, err)
	assert.Equal(t, limit, newLimit)
}

// XXX quota file conflicts? - probably not really possible

func TestRestTreeDeleteNotFoundPath(t *testing.T) {
	testDirPath, _, cleanup := requireCluster(t)
	defer cleanup(t)

	err := testConnection.TreeDeleteCreate(testDirPath + "/blah")
	assert.NoError(t, err)
}

func TestRestVersion(t *testing.T) {
	_, _, cleanup := requireCluster(t)
	defer cleanup(t)

	info, err := testConnection.GetVersionInfo()
	assert.NoError(t, err)

	version, err := info.GetSemanticVersion()
	assert.NoError(t, err)

	if version.LT(semver.Version{Major: 4, Minor: 3, Patch: 0}) {
		t.Fatalf("Too small test version %v", version)
	}
}

func TestRestChmodNotFound(t *testing.T) {
	testDirPath, _, cleanup := requireCluster(t)
	defer cleanup(t)

	path := testDirPath + "/foo"

	_, err := testConnection.FileChmod(path, "0555")
	assertRestError(t, err, 404, "fs_no_such_entry_error")
}

func TestRestChmodByPath(t *testing.T) {
	testDirPath, _, cleanup := requireCluster(t)
	defer cleanup(t)

	attributes, err := testConnection.FileChmod(testDirPath, "0555")
	assert.NoError(t, err)
	assert.Equal(t, attributes.Mode, "0555")

	attributes, err = testConnection.FileChmod(testDirPath, "0777")
	assert.NoError(t, err)
	assert.Equal(t, attributes.Mode, "0777")
}

func TestRestChmodById(t *testing.T) {
	_, testDirId, cleanup := requireCluster(t)
	defer cleanup(t)

	attributes, err := testConnection.FileChmod(testDirId, "0555")
	assert.NoError(t, err)
	assert.Equal(t, attributes.Mode, "0555")

	attributes, err = testConnection.FileChmod(testDirId, "0777")
	assert.NoError(t, err)
	assert.Equal(t, attributes.Mode, "0777")
}

func TestRestGetExportNotFoundPath(t *testing.T) {
	_, _, cleanup := requireCluster(t)
	defer cleanup(t)

	_, err := testConnection.ExportGet("/blahhhhhhh")
	assertRestError(t, err, 404, "nfs_export_doesnt_exist_error")
}

func TestRestGetExportNotFoundId(t *testing.T) {
	_, _, cleanup := requireCluster(t)
	defer cleanup(t)

	_, err := testConnection.ExportGet("999999")
	assertRestError(t, err, 404, "nfs_export_doesnt_exist_error")
}

func TestRestGetExportDefaultPath(t *testing.T) {
	_, _, cleanup := requireCluster(t)
	defer cleanup(t)

	export, err := testConnection.ExportGet("/")
	assert.NoError(t, err)
	assert.Equal(t, export, ExportResponse{"1", "/", "/"})
}

func TestRestGetExportDefaultId(t *testing.T) {
	_, _, cleanup := requireCluster(t)
	defer cleanup(t)

	export, err := testConnection.ExportGet("1")
	assert.NoError(t, err)
	assert.Equal(t, export, ExportResponse{"1", "/", "/"})
}

func TestRestCreateDeleteExport(t *testing.T) {
	testDirPath, _, cleanup := requireCluster(t)
	defer cleanup(t)

	exportPath := "/some/export"

	export, err := testConnection.ExportCreate(exportPath, testDirPath)
	assert.NoError(t, err)
	assert.Equal(t, export.ExportPath, exportPath)
	assert.Equal(t, export.FsPath, testDirPath)

	err = testConnection.ExportDelete(export.ExportPath)
	assert.NoError(t, err)

	_, err = testConnection.ExportGet(export.ExportPath)
	assertRestError(t, err, 404, "nfs_export_doesnt_exist_error")
}
