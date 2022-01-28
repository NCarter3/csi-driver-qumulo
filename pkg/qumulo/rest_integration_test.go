package qumulo

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	nocleanup   bool
	host        string
	port        int
	username    string
	password    string
	logging     bool

	connection *Connection
	fixturedir  string
	testnum     int
)

func TestMain(m *testing.M) {
	flag.BoolVar  (&nocleanup, "nocleanup", false,      "Skip clean up of artifacts")
	flag.StringVar(&host,      "host",      "",         "Host to connect to")
	flag.IntVar   (&port,      "port",      8000,       "Port to connect to")
	flag.StringVar(&username,  "username",  "admin",    "Username to connect as")
	flag.StringVar(&password,  "password",  "Admin123", "Password to use")
	flag.BoolVar  (&logging,   "logging",   false,      "Enable logging")
	flag.Parse()

	if !logging {
		log.SetOutput(ioutil.Discard)
	}

	if len(host) != 0 {

		c := MakeConnection(host, port, username, password, new(http.Client))

		_, err := c.CreateDir("/", "gotest")
		if err != nil {
			panic(err)
		}

		fixturedir = "/gotest"

		connection = &c
	}

	code := m.Run()

	if connection != nil && !nocleanup {
		err := connection.TreeDeleteCreate(fixturedir)
		if err != nil {
			log.Printf("Failed to clean up test dir %q with tree delete: %v", fixturedir, err)
			code = 1
		}
	}

	os.Exit(code)
}

func setupTest(t *testing.T) (testDirPath string, testDirId string, cleanup func(t *testing.T)) {
	if connection == nil {
		t.Skip("requires qumulo server")
		return
	}

	name := fmt.Sprintf("testnum-%d", testnum)

	attributes, err := connection.CreateDir(fixturedir, name)
	if err != nil {
		t.Fatalf("Error creating subdir %s/%s: %v", fixturedir, name, err)
		return
	}

	testnum += 1

	testDirPath = fmt.Sprintf("%s/%s", fixturedir, name)
	testDirId = attributes.Id

	cleanup = func(t *testing.T) {
		if !t.Failed() {
			err := connection.TreeDeleteCreate(testDirPath)
			assert.NoError(t, err)
		}
	}

	return
}

/*  _            _
 * | |_ ___  ___| |_ ___
 * | __/ _ \/ __| __/ __|
 * | ||  __/\__ \ |_\__ \
 *  \__\___||___/\__|___/
 *  FIGLET: tests
 */

func TestRestCreateDir(t *testing.T) {
	testDirPath, _, cleanup := setupTest(t)
	defer cleanup(t)

	attributes, err := connection.CreateDir(testDirPath, "bar")
	assert.NoError(t, err)
	if attributes.Type != "FS_FILE_TYPE_DIRECTORY" {
		t.Fatalf("unexpected attributes %v", attributes)
	}
}

func TestRestCreateDirTwiceErrors(t *testing.T) {
	testDirPath, _, cleanup := setupTest(t)
	defer cleanup(t)

	_, err := connection.CreateDir(testDirPath, "bar")
	assert.NoError(t, err)

	_, err = connection.CreateDir(testDirPath, "bar")
	assertRestError(t, err, 409, "fs_entry_exists_error")
}

func TestRestEnsureDirNewDir(t *testing.T) {
	testDirPath, _, cleanup := setupTest(t)
	defer cleanup(t)

	attributes, err := connection.EnsureDir(testDirPath, "somedir")
	assert.NoError(t, err)
	if attributes.Type != "FS_FILE_TYPE_DIRECTORY" {
		t.Fatalf("unexpected attributes %v", attributes)
	}
}

func TestRestEnsureDirAfterCreateDir(t *testing.T) {
	testDirPath, _, cleanup := setupTest(t)
	defer cleanup(t)

	attributes1, err := connection.EnsureDir(testDirPath, "somedir")
	assert.NoError(t, err)

	attributes2, err := connection.EnsureDir(testDirPath, "blah")
	assert.NoError(t, err)

	if attributes1 != attributes1 {
		t.Fatalf("unexpected attributes mismatch %v != %v", attributes1, attributes2)
	}
}

func TestRestEnsureDirTwice(t *testing.T) {
	testDirPath, _, cleanup := setupTest(t)
	defer cleanup(t)

	attributes1, err := connection.EnsureDir(testDirPath, "blah")
	assert.NoError(t, err)

	attributes2, err := connection.EnsureDir(testDirPath, "blah")
	assert.NoError(t, err)

	if attributes1 != attributes1 {
		t.Fatalf("unexpected attributes mismatch %v != %v", attributes1, attributes2)
	}
}

func TestRestCreateFile(t *testing.T) {
	testDirPath, _, cleanup := setupTest(t)
	defer cleanup(t)

	attributes, err := connection.CreateFile(testDirPath, "notadir")
	assert.NoError(t, err)
	if attributes.Type != "FS_FILE_TYPE_FILE" {
		t.Fatalf("unexpected attributes %v", attributes)
	}
}

func TestRestEnsureDirWithFileConflict(t *testing.T) {
	testDirPath, _, cleanup := setupTest(t)
	defer cleanup(t)

	_, err := connection.CreateFile(testDirPath, "x")
	assert.NoError(t, err)

	_, err = connection.EnsureDir(testDirPath, "x")
	path := fmt.Sprintf("%s/x", testDirPath)
	assert.EqualError(t, err, fmt.Sprintf("A non-directory exists at: %q, FS_FILE_TYPE_FILE", path))
}

func TestRestCreateQuota(t *testing.T) {
	_, testDirId, cleanup := setupTest(t)
	defer cleanup(t)

	newLimit := uint64(1024 * 1024 * 1024)
	err := connection.CreateQuota(testDirId, newLimit)
	assert.NoError(t, err)

	limit, err := connection.GetQuota(testDirId)
	assert.NoError(t, err)
	assert.Equal(t, limit, newLimit)
}

func TestRestCreateQuotaTwiceErrors(t *testing.T) {
	_, testDirId, cleanup := setupTest(t)
	defer cleanup(t)

	err := connection.CreateQuota(testDirId, 1024 * 1024 * 1024)
	assert.NoError(t, err)

	err = connection.CreateQuota(testDirId, 1024 * 1024 * 1024)
	assertRestError(t, err, 409, "api_quotas_quota_limit_already_set_error")
}

func TestRestUpdateQuotaNoQuotaErrors(t *testing.T) {
	_, testDirId, cleanup := setupTest(t)
	defer cleanup(t)

	err := connection.UpdateQuota(testDirId, 1024 * 1024 * 1024)
	assertRestError(t, err, 404, "api_quotas_quota_limit_not_found_error")
}

func TestRestUpdateQuotaAfterCreateQuota(t *testing.T) {
	_, testDirId, cleanup := setupTest(t)
	defer cleanup(t)

	newLimit := uint64(1024 * 1024 * 1024)
	err := connection.CreateQuota(testDirId, newLimit)
	assert.NoError(t, err)

	err = connection.UpdateQuota(testDirId, newLimit)
	assert.NoError(t, err)

	limit, err := connection.GetQuota(testDirId)
	assert.NoError(t, err)
	assert.Equal(t, limit, newLimit)
}

func TestRestEnsureQuotaNewQuota(t *testing.T) {
	_, testDirId, cleanup := setupTest(t)
	defer cleanup(t)

	newLimit := uint64(1024 * 1024 * 1024)
	err := connection.EnsureQuota(testDirId, newLimit)
	assert.NoError(t, err)

	limit, err := connection.GetQuota(testDirId)
	assert.NoError(t, err)
	assert.Equal(t, limit, newLimit)
}

func TestRestEnsureQuotaAfterCreateQuota(t *testing.T) {
	_, testDirId, cleanup := setupTest(t)
	defer cleanup(t)

	err := connection.CreateQuota(testDirId, 1024 * 1024 * 1024)
	assert.NoError(t, err)

	newLimit := uint64(2 * 1024 * 1024 * 1024)
	err = connection.EnsureQuota(testDirId, newLimit)
	assert.NoError(t, err)

	limit, err := connection.GetQuota(testDirId)
	assert.NoError(t, err)
	assert.Equal(t, limit, newLimit)
}

func TestRestEnsureQuotaTwice(t *testing.T) {
	_, testDirId, cleanup := setupTest(t)
	defer cleanup(t)

	newLimit := uint64(2 * 1024 * 1024 * 1024)
	err := connection.EnsureQuota(testDirId, newLimit)
	assert.NoError(t, err)

	err = connection.EnsureQuota(testDirId, newLimit)
	assert.NoError(t, err)

	limit, err := connection.GetQuota(testDirId)
	assert.NoError(t, err)
	assert.Equal(t, limit, newLimit)
}

// XXX quota file conflicts? - probably not really possible

func TestRestTreeDeleteNotFoundPath(t *testing.T) {
	testDirPath, _, cleanup := setupTest(t)
	defer cleanup(t)

	err := connection.TreeDeleteCreate(testDirPath + "/blah")
	assert.NoError(t, err)
}
