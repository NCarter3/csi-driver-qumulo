package qumulo

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func assertRestError(t *testing.T, err error, expectedStatus int, expectedErrorClass string) {
	if err == nil {
		t.Fatal("unexpected nil error")
	}
	switch err.(type) {
	case RestError:
		z := err.(RestError)
		if z.StatusCode != expectedStatus {
			t.Fatalf("error status %d != %d: %v", expectedStatus, z.StatusCode, z)
		}
		if z.ErrorClass != expectedErrorClass {
			t.Fatalf("error class %q does not match %q: %q", expectedErrorClass, z.ErrorClass, z)
		}
	default:
		t.Fatalf("unexpected error: %v", err)
	}
}

var (
	testHost     string
	testPort     int
	testUsername string
	testPassword string

	testConnection *Connection
	testFixtureDir string
	testNumber     int
)

func TestMain(m *testing.M) {

	// Get cluster connection settings from the environment first then allow override
	// with flags. An empty host indicates no cluster which bypassess tests using requireCluster.

	testHost = os.Getenv("QUMULO_TEST_HOST")
	portStr := os.Getenv("QUMULO_TEST_PORT")
	testUsername = os.Getenv("QUMULO_TEST_USERNAME")
	testPassword = os.Getenv("QUMULO_TEST_PASSWORD")
	testroot := os.Getenv("QUMULO_TEST_ROOT")

	var nocleanup bool
	var logging bool

	flag.StringVar(&testHost, "host", testHost, "Host to connect to")
	flag.StringVar(&portStr, "port", portStr, "Port to connect to")
	flag.StringVar(&testUsername, "username", testUsername, "Username to connect as")
	flag.StringVar(&testPassword, "password", testPassword, "Password to use")
	flag.StringVar(&testroot, "testroot", testroot, "Root directory to put test dir in")
	flag.BoolVar(&nocleanup, "nocleanup", false, "Skip clean up of artifacts")
	flag.BoolVar(&logging, "logging", false, "Enable logging")

	flag.Parse()

	if len(testHost) != 0 {
		var err error
		testPort, err = strconv.Atoi(portStr)
		if err != nil {
			log.Fatal(err)
		}

		if testPort == 0 {
			log.Fatal("QUMULO_TEST_PORT is required with QUMULO_TEST_HOST")
		}
		if len(testUsername) == 0 {
			log.Fatal("QUMULO_TEST_USERNAME is required with QUMULO_TEST_HOST")
		}
		if len(testPassword) == 0 {
			log.Fatal("QUMULO_TEST_PASSWORD is required with QUMULO_TEST_HOST")
		}

		c := MakeConnection(testHost, testPort, testUsername, testPassword, new(http.Client))

		if len(testroot) == 0 {
			testroot = "/"
		}

		_, err = c.CreateDir(testroot, "gotest")
		if err != nil {
			log.Fatal(err)
		}

		testFixtureDir = fmt.Sprintf("%s/gotest", testroot)

		testConnection = &c
	}

	if !logging {
		log.SetOutput(ioutil.Discard)
	}

	code := m.Run()

	if testConnection != nil && !nocleanup {
		err := testConnection.TreeDeleteCreate(testFixtureDir)
		if err != nil {
			log.Printf("Failed to clean up test dir %q with tree delete: %v", testFixtureDir, err)
			code = 1
		}
	}

	os.Exit(code)
}

func requireCluster(t *testing.T) (testDirPath string, testDirId string, cleanup func(t *testing.T)) {
	if testConnection == nil {
		t.Skip("requires qumulo server")
		return
	}

	name := fmt.Sprintf("testNumber-%d", testNumber)

	attributes, err := testConnection.CreateDir(testFixtureDir, name)
	if err != nil {
		t.Fatalf("Error creating subdir %s/%s: %v", testFixtureDir, name, err)
		return
	}

	testNumber += 1

	testDirPath = fmt.Sprintf("%s/%s", testFixtureDir, name)
	testDirId = attributes.Id

	cleanup = func(t *testing.T) {
		if !t.Failed() {
			err := testConnection.TreeDeleteCreate(testDirPath)
			assert.NoError(t, err)
		}
	}

	return
}
