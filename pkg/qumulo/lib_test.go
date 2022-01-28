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
	connection *Connection
	fixturedir  string
	testnum     int
)

func TestMain(m *testing.M) {

	// Get cluster connection settings from the environment first then allow override
	// with flags. An empty host indicates no cluster which bypassess tests using requireCluster.

	host     := os.Getenv("QUMULO_TEST_HOST")
	portStr  := os.Getenv("QUMULO_TEST_PORT")
	username := os.Getenv("QUMULO_TEST_USERNAME")
	password := os.Getenv("QUMULO_TEST_PASSWORD")

	var nocleanup bool
	var logging   bool

	flag.StringVar(&host,      "host",      host,       "Host to connect to")
	flag.StringVar(&portStr,   "port",      portStr,    "Port to connect to")
	flag.StringVar(&username,  "username",  username,   "Username to connect as")
	flag.StringVar(&password,  "password",  password,   "Password to use")
	flag.BoolVar  (&nocleanup, "nocleanup", false,      "Skip clean up of artifacts")
	flag.BoolVar  (&logging,   "logging",   false,      "Enable logging")

	flag.Parse()

	if !logging {
		log.SetOutput(ioutil.Discard)
	}

	if len(host) != 0 {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			log.Fatal(err)
		}

		if port == 0 {
			log.Fatal("QUMULO_TEST_PORT is required with QUMULO_TEST_HOST");
		}
		if len(username) == 0 {
			log.Fatal("QUMULO_TEST_USERNAME is required with QUMULO_TEST_HOST");
		}
		if len(password) == 0 {
			log.Fatal("QUMULO_TEST_PASSWORD is required with QUMULO_TEST_HOST");
		}

		c := MakeConnection(host, port, username, password, new(http.Client))

		_, err = c.CreateDir("/", "gotest")
		if err != nil {
			log.Fatal(err)
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

func requireCluster(t *testing.T) (testDirPath string, testDirId string, cleanup func(t *testing.T)) {
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

