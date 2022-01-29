package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/kubernetes-csi/csi-driver-qumulo/pkg/qumulo"
)

func main() {
	hostPtr := flag.String("host", "localhost", "Host to connect to")
	portPtr := flag.Int("port", 8000, "Port to connect to")
	username := flag.String("username", "admin", "Username to connect as")
	password := flag.String("password", "", "Password to use")
	logging := flag.Bool("logging", false, "Enable logging")

	flag.Parse()

	if !*logging {
		log.SetOutput(ioutil.Discard)
	}

	verb := strings.ToUpper(flag.Args()[0])
	uri := flag.Args()[1]

	connection := qumulo.MakeConnection(*hostPtr, *portPtr, *username, *password, new(http.Client))

	requestBody := []byte{}

	if verb == "PUT" || verb == "POST" {
		var err error
		requestBody, err = ioutil.ReadAll(os.Stdin)
		if err != nil {
			log.Fatal(err)
		}
	}

	responseData, err := connection.Do(verb, uri, requestBody)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(responseData))
}
