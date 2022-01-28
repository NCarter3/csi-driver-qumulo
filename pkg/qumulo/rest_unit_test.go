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
	Uri              string
	StatusCode       int
	BodyIn			 string
	BodyOut          string
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


	return &http.Response {
		StatusCode: message.StatusCode,
		Body:       ioutil.NopCloser(bytes.NewBufferString(message.BodyOut)),
		Header:     make(http.Header),
	}, nil
}

func newTestClient(t *testing.T, host string, port int, messages *[]Message) *http.Client {
	return &http.Client {
		Transport: &FakeTransport{t, host, port, messages},
	}
}

const (
	testHost = "1.2.3.4"
	testPort = 44
)

/*  _            _
 * | |_ ___  ___| |_ ___
 * | __/ _ \/ __| __/ __|
 * | ||  __/\__ \ |_\__ \
 *  \__\___||___/\__|___/
 *  FIGLET: tests
 */

func TestRestAutoLoginSucces(t *testing.T) {
	messages := []Message {
		{ "/hi", 401, "", "" },
		{ "/v1/session/login", 200, "{\"username\":\"bob\",\"password\":\"yeruncle\"}", ""},
		{ "/hi", 200, "", "" },
		{ "/bye", 200, "", "" },
	}
	client := newTestClient(t, testHost, testPort, &messages)

	connection := MakeConnection(testHost, testPort, "bob", "yeruncle", client)
	_, err := connection.Get("/hi")
	assert.NoError(t, err)
	_, err = connection.Get("/bye")
	assert.NoError(t, err)

	assertMessagesConsumed(t, messages)
}

func TestRestAutoLoginFail(t *testing.T) {
	messages := []Message {
		{ "/hi", 401, "", "" },
		{ "/v1/session/login", 201, "{\"username\":\"bob\",\"password\":\"yeruncle\"}", ""},
	}
	client := newTestClient(t, testHost, testPort, &messages)

	connection := MakeConnection(testHost, testPort, "bob", "yeruncle", client)
	_, err := connection.Get("/hi")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Login failed: ")

	assertMessagesConsumed(t, messages)
}

/*  _       _                       _   _
 * (_)_ __ | |_ ___  __ _ _ __ __ _| |_(_) ___  _ __
 * | | '_ \| __/ _ \/ _` | '__/ _` | __| |/ _ \| '_ \
 * | | | | | ||  __/ (_| | | | (_| | |_| | (_) | | | |
 * |_|_| |_|\__\___|\__, |_|  \__,_|\__|_|\___/|_| |_|
 *                  |___/
 *  FIGLET: integration
 */

