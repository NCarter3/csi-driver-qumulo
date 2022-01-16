package qumulo

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
)

type LoginRequest struct {
	Username   string   `json:"username"`
	Password   string   `json:"password"`
}

type Connection struct {
	Host         string
	Port         int
	Username     string
	Password     string
	token        string
	client		 *http.Client
}

func MakeConnection(host string, port int, username string, password string) Connection {
	// XXX scott: figure out how to make this an option, or install certs somehow
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	c := new(http.Client)
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	return Connection{Host: host, Port: port, Username: username, Password: password, client: c}
}

type RestError struct {
	StatusCode  int
	Description string
	Module      string
	ErrorClass  string
	Stack       []string
	UserVisible bool
}

func (e RestError) Error() string {
	return fmt.Sprintf(
		"%d %s %s %s %s",
		e.StatusCode,
		e.Description,
		e.Module,
		e.ErrorClass,
		e.Stack,
	)
}

type ErrorResponse struct {
	Description       string   `json:"description"`
	Module            string   `json:"module"`
	ErrorClass        string   `json:"error_class"`
	UserVisible       bool     `json:"user_visible"`
	Stack             []string `json:"stack"`
}

func MakeRestError(status int, response []byte) RestError {
	var obj ErrorResponse
	json.Unmarshal(response, &obj)
	return RestError{
		StatusCode:  status,
		Description: obj.Description,
		Module:      obj.Module,
		ErrorClass:  obj.ErrorClass,
		Stack:       obj.Stack,
		UserVisible: obj.UserVisible,
	}
}

func (self *Connection) Login() {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	loginUrl := fmt.Sprintf("https://%s:%d/v1/session/login", self.Host, self.Port)

	body := new(LoginRequest)
	body.Username = self.Username
	body.Password = self.Password

	json_data, err := json.Marshal(body)
	if err != nil {
		log.Fatal(err)
	}

	response, err := self.client.Post(loginUrl, "application/json", bytes.NewBuffer(json_data))
	if err != nil {
		log.Fatal(err)
	}

	if response.StatusCode != 200 {
		log.Fatalf("Login failed %s", response.Status)
	}

	var res map[string]string

	json.NewDecoder(response.Body).Decode(&res)

	self.token = res["bearer_token"]
}

func (self *Connection) do(verb string, uri string, body []byte) ([]byte, error) {
	url := fmt.Sprintf("https://%s:%d%s", self.Host, self.Port, uri)
	req, err := http.NewRequest(verb, url, nil)
	req.Header.Add("Authorization", "Bearer "+self.token)

	if len(body) > 0 {
		req.Body = io.NopCloser(bytes.NewBuffer(body))
		req.Header.Add("Content-Type", "application/json")
	}

	response, err := self.client.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	status := response.StatusCode

	log.Print(response.StatusCode)
	log.Print(response.Header)

	responseData, err := ioutil.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		log.Fatal(err)
	}

	if status < 200 || status >= 300 {
		return nil, MakeRestError(status, responseData)
	}

	return responseData, err
}

func (self *Connection) Do(verb string, uri string, body []byte) (result []byte, err error) {
	result, err = self.do(verb, uri, body)

	if err == nil {
		return
	}

	switch err.(type) {
	case RestError:
		z := err.(RestError)
		if z.StatusCode != 401 {
			return
		}
	default:
		return
	}

	log.Print("401 reauthenticating")
	self.Login()

	result, err = self.do(verb, uri, body)

	return
}

func (self *Connection) Get(uri string) (result []byte, err error) {
	return self.Do("GET", uri, []byte{})
}

func (self *Connection) Post(uri string, body []byte) (result []byte, err error) {
	return self.Do("POST", uri, body)
}

func (self *Connection) Put(uri string, body []byte) (result []byte, err error) {
	return self.Do("PUT", uri, body)
}

func (self *Connection) Patch(uri string, body []byte) (result []byte, err error) {
	return self.Do("PATCH", uri, body)
}

func (self *Connection) Delete(uri string) (result []byte, err error) {
	return self.do("DELETE", uri, []byte{})
}

/*   ____                _       ____  _
 *  / ___|_ __ ___  __ _| |_ ___|  _ \(_)_ __
 * | |   | '__/ _ \/ _` | __/ _ \ | | | | '__|
 * | |___| | |  __/ (_| | ||  __/ |_| | | |
 *  \____|_|  \___|\__,_|\__\___|____/|_|_|
 *  FIGLET: CreateDir
 */

type CreateDirRequest struct {
	Name              string `json:"name"`
	Action            string `json:"action"`
}

func (self *Connection) CreateDir(path string, name string) (id string, err error) {
	uri := fmt.Sprintf("/v1/files/%s/entries/", url.QueryEscape(path))

	body := new(CreateDirRequest)
	body.Name = name
	body.Action = "CREATE_DIRECTORY"

	json_data, err := json.Marshal(body)
	if err != nil {
		log.Fatal(err)
	}

	responseData, err := self.Post(uri, json_data)
	if err != nil {
		return
	}

	var result map[string]interface{}
	json.Unmarshal(responseData, &result)

	id = result["id"].(string)

	return
}

/*   ___              _
 *  / _ \ _   _  ___ | |_ __ _ ___
 * | | | | | | |/ _ \| __/ _` / __|
 * | |_| | |_| | (_) | || (_| \__ \
 *  \__\_\\__,_|\___/ \__\__,_|___/
 *  FIGLET: Quotas
 */

type CreateQuotaRequest struct {
	Id                string `json:"id"`
	Limit             string `json:"limit"`
}

func (self *Connection) CreateQuota(id string, limit uint64) (err error) {
	uri := "/v1/files/quotas/"

	body := new(CreateQuotaRequest)
	body.Id = id
	body.Limit = strconv.FormatUint(limit, 10)

	json_data, err := json.Marshal(body)
	if err != nil {
		return
	}

	_, err = self.Post(uri, json_data)

	return
}

func (self *Connection) UpdateQuota(id string, limit uint64) (err error) {
	uri := fmt.Sprintf("/v1/files/quotas/%s", id)

	body := new(CreateQuotaRequest)
	body.Id = id
	body.Limit = strconv.FormatUint(limit, 10)

	json_data, err := json.Marshal(body)
	if err != nil {
		return
	}

	_, err = self.Put(uri, json_data)

	return
}


/*  ____                 _           ____       _   _
 * |  _ \ ___  ___  ___ | |_   _____|  _ \ __ _| |_| |__
 * | |_) / _ \/ __|/ _ \| \ \ / / _ \ |_) / _` | __| '_ \
 * |  _ <  __/\__ \ (_) | |\ V /  __/  __/ (_| | |_| | | |
 * |_| \_\___||___/\___/|_| \_/ \___|_|   \__,_|\__|_| |_|
 *  FIGLET: ResolvePath
 */

func (self *Connection) ResolvePath(path string) (id string, err error) {
	uri := fmt.Sprintf("/v1/files/%s/info/attributes", url.QueryEscape(path))

	responseData, err := self.Get(uri)
	if err != nil {
		return
	}

	var result map[string]interface{}
	json.Unmarshal(responseData, &result)

	id = result["id"].(string)

	return
}

/*  _____              ____       _      _        ____                _
 * |_   _| __ ___  ___|  _ \  ___| | ___| |_ ___ / ___|_ __ ___  __ _| |_ ___
 *   | || '__/ _ \/ _ \ | | |/ _ \ |/ _ \ __/ _ \ |   | '__/ _ \/ _` | __/ _ \
 *   | || | |  __/  __/ |_| |  __/ |  __/ ||  __/ |___| | |  __/ (_| | ||  __/
 *   |_||_|  \___|\___|____/ \___|_|\___|\__\___|\____|_|  \___|\__,_|\__\___|
 *  FIGLET: TreeDeleteCreate
 */

type TreeDeleteCreateRequest struct {
	Id                string `json:"id"`
}

func (self *Connection) TreeDeleteCreate(path string) (err error) {
	id, err := self.ResolvePath(path)
	if err != nil {
		return
	}

	uri := "/v1/tree-delete/jobs/"

	body := new(TreeDeleteCreateRequest)
	body.Id = id

	json_data, err := json.Marshal(body)
	if err != nil {
		return
	}

	_, err = self.Post(uri, json_data)

	return err
}
