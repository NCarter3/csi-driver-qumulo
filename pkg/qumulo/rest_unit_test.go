package qumulo

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

type Message struct {
	Uri        string
	StatusCode int
	BodyIn     string
	BodyOut    string
}

func assertMessagesConsumed(t *testing.T, messages []Message) {
	if len(messages) != 0 {
		t.Fatalf("not all messages used by test: %v", messages)
	}
}

type FakeTransport struct {
	test     *testing.T
	host     string
	port     int
	messages *[]Message
}

func (self *FakeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	message := (*self.messages)[0]
	*self.messages = (*self.messages)[1:]

	expectedUrl := fmt.Sprintf("https://%s:%d%s", self.host, self.port, message.Uri)
	if req.URL.String() != expectedUrl {
		self.test.Fatalf("unexpected url %v != %s", req.URL, expectedUrl)
	}

	if req.Body != nil {
		body, err := ioutil.ReadAll(req.Body)
		req.Body.Close()
		if err != nil {
			self.test.Fatalf("assert %v", err)
		}
		if string(body) != message.BodyIn {
			self.test.Fatalf("unexpected body %v != %s", string(body), message.BodyIn)
		}
	}

	return &http.Response{
		StatusCode: message.StatusCode,
		Body:       ioutil.NopCloser(bytes.NewBufferString(message.BodyOut)),
		Header:     make(http.Header),
	}, nil
}

func newTestClient(t *testing.T, host string, port int, messages *[]Message) *http.Client {
	return &http.Client{
		Transport: &FakeTransport{t, host, port, messages},
	}
}

/*  _            _
 * | |_ ___  ___| |_ ___
 * | __/ _ \/ __| __/ __|
 * | ||  __/\__ \ |_\__ \
 *  \__\___||___/\__|___/
 *  FIGLET: tests
 */

func TestRestAutoLoginSucces(t *testing.T) {
	messages := []Message{
		{"/hi", 401, "", ""},
		{"/v1/session/login", 200, "{\"username\":\"bob\",\"password\":\"yeruncle\"}", ""},
		{"/hi", 200, "", ""},
		{"/bye", 200, "", ""},
	}
	client := newTestClient(t, "1.2.3.4", 44, &messages)

	connection := MakeConnection("1.2.3.4", 44, "bob", "yeruncle", client)
	_, err := connection.Get("/hi")
	assert.NoError(t, err)
	_, err = connection.Get("/bye")
	assert.NoError(t, err)

	assertMessagesConsumed(t, messages)
}

func TestRestAutoLoginFail(t *testing.T) {
	messages := []Message{
		{"/hi", 401, "", ""},
		{"/v1/session/login", 201, "{\"username\":\"bob\",\"password\":\"yeruncle\"}", ""},
	}
	client := newTestClient(t, "1.2.3.4", 44, &messages)

	connection := MakeConnection("1.2.3.4", 44, "bob", "yeruncle", client)
	_, err := connection.Get("/hi")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Login failed: ")

	assertMessagesConsumed(t, messages)
}

func TestRestSemanticVersionBadRevsion1(t *testing.T) {
	info := QumuloVersionInfo{Revision: "blah"}
	_, err := info.GetSemanticVersion()
	assert.EqualError(t, err, "Could not decode version &{\"blah\"}")
}

func TestRestSemanticVersionBadRevsion2(t *testing.T) {
	info := QumuloVersionInfo{Revision: "Qumulo Core aasdfa"}
	_, err := info.GetSemanticVersion()
	assert.EqualError(t, err, "No Major.Minor.Patch elements found")
}

func TestRestSemanticVersionHappy(t *testing.T) {
	info1 := QumuloVersionInfo{Revision: "Qumulo Core 2.5.1"}
	info2 := QumuloVersionInfo{Revision: "Qumulo Core 1.2.3"}

	v1, err := info1.GetSemanticVersion()
	assert.NoError(t, err)
	assert.Equal(t, v1.String(), "2.5.1")

	v2, err := info2.GetSemanticVersion()
	assert.NoError(t, err)
	assert.Equal(t, v2.String(), "1.2.3")

	if !v2.LT(v1) {
		t.Fatalf("Unexpected version ordering %v !< %v", v2, v1)
	}
}
