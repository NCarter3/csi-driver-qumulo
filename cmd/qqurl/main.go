package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/kubernetes-csi/csi-driver-qumulo/pkg/qumulo"
	"k8s.io/klog/v2"
)

func main() {
	hostPtr := flag.String("host", "localhost", "Host to connect to")
	portPtr := flag.Int("port", 8000, "Port to connect to")
	username := flag.String("username", "admin", "Username to connect as")
	password := flag.String("password", "", "Password to use")
	logging := flag.Bool("logging", false, "Enable logging")

	flag.Parse()

	if !*logging {
		vlogFlags := &flag.FlagSet{}
		klog.InitFlags(vlogFlags)
		klog.SetOutput(ioutil.Discard)
		vlogFlags.Set("logtostderr", "false")
		vlogFlags.Set("alsologtostderr", "false")
	} else {
		vlogFlags := &flag.FlagSet{}
		klog.InitFlags(vlogFlags)
		vlogFlags.Set("stderrthreshold", "INFO")
		vlogFlags.Set("v", "3")
	}

	verb := strings.ToUpper(flag.Args()[0])
	uri := flag.Args()[1]

	connection := qumulo.MakeConnection(*hostPtr, *portPtr, *username, *password, new(http.Client))

	requestBody := []byte{}

	if verb == "PUT" || verb == "POST" {
		var err error
		requestBody, err = ioutil.ReadAll(os.Stdin)
		if err != nil {
			klog.Fatal(err)
		}
	}

	responseData, err := connection.Do(verb, uri, requestBody)
	if err != nil {
		klog.Fatal(err)
	}

	fmt.Println(string(responseData))
}
