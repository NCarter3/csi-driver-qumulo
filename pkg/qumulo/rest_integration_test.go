package qumulo

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"testing"
)

var (
	host        string
	port        int
	username    string
	password    string
	connection *Connection
	fixturedir  string
	testnum     int
)

func TestMain(m *testing.M) {
	flag.StringVar(&host,     "host",     "",         "Host to connect to")
	flag.IntVar   (&port,     "port",     8000,       "Port to connect to")
	flag.StringVar(&username, "username", "admin",    "Username to connect as")
	flag.StringVar(&password, "password", "Admin123", "Password to use")
	flag.Parse()

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

	if connection != nil && code == 0 {
		err := connection.TreeDeleteCreate(fixturedir)
		if err != nil {
			log.Printf("Failed to clean up test dir %q with tree delete: %v", fixturedir, err)
			code = 1
		}
	}

	os.Exit(code)
}

func setupTest(t *testing.T) (testdir string, cleanup func(t *testing.T)) {
	if connection == nil {
		t.Skip("requires qumulo server")
		return
	}

	name := fmt.Sprintf("testnum-%d", testnum)

	_, err := connection.CreateDir(fixturedir, name)
	if err != nil {
		t.Fatalf("Error creating subdir %s/%s: %v", fixturedir, name, err)
		return
	}

	testnum += 1

	testdir = fmt.Sprintf("%s/%s", fixturedir, name)

	cleanup = func(t *testing.T) {
		if !t.Failed() {
			err := connection.TreeDeleteCreate(testdir)
			assertNoError(t, err)
		}
	}

	return
}

func TestRestSmoke(t *testing.T) {
	testdir, cleanup := setupTest(t)
	defer cleanup(t)

	id, err := connection.CreateDir(testdir, "bar")
	assertNoError(t, err)

	t.Logf("Created Dir %v", id)
}

func TestRestCreateDir(t *testing.T) {
	testdir, cleanup := setupTest(t)
	defer cleanup(t)

	id, err := connection.CreateDir(testdir, "bar")
	assertNoError(t, err)
	t.Logf("Created Dir %v", id)

	id, err = connection.CreateDir(testdir, "bar")
	assertRestError(t, err, 409, "fs_entry_exists_error")
}
