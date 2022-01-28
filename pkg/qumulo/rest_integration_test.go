package qumulo

import (
	"fmt"
	"testing"

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
	path := fmt.Sprintf("%s/x", testDirPath)
	assert.EqualError(t, err, fmt.Sprintf("A non-directory exists at: %q, FS_FILE_TYPE_FILE", path))
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

	err := testConnection.CreateQuota(testDirId, 1024 * 1024 * 1024)
	assert.NoError(t, err)

	err = testConnection.CreateQuota(testDirId, 1024 * 1024 * 1024)
	assertRestError(t, err, 409, "api_quotas_quota_limit_already_set_error")
}

func TestRestUpdateQuotaNoQuotaErrors(t *testing.T) {
	_, testDirId, cleanup := requireCluster(t)
	defer cleanup(t)

	err := testConnection.UpdateQuota(testDirId, 1024 * 1024 * 1024)
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

	err := testConnection.CreateQuota(testDirId, 1024 * 1024 * 1024)
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
