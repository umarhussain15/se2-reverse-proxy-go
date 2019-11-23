package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

// NginxHostPath will hold the url of nginx instance. e.g http://nginx
var NginxHostPath string

// proxyHandler will call either dynamicContentProxy or staticContentProxy based on the prefix "/library" presence
// in the path of the request url.
func proxyHandler(res http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	if strings.HasPrefix(path, "/library") {
		dynamicContentProxy(res, req)
	} else {
		staticContentProxy(res, req)
	}
}

func dynamicContentProxy(res http.ResponseWriter, req *http.Request) {
	log.Printf("dynamic request handler %s", req.URL.Path)
	//TODO: forward request to gserve instances based on round robin load balancing
	serverUrl, _ := url.Parse("http://gserve1:8081")
	httputil.NewSingleHostReverseProxy(serverUrl).ServeHTTP(res, req)
}

// staticContentProxy will use reverse proxy with host url of our nginx instance. nginx will respond to the request and
// go will send back the result it received from nginx.
func staticContentProxy(res http.ResponseWriter, req *http.Request) {
	log.Printf("proxying to nginx for: %s", req.URL.Path)
	serverUrl, _ := url.Parse(NginxHostPath)
	httputil.NewSingleHostReverseProxy(serverUrl).ServeHTTP(res, req)
}

// serverAddress reads environment for port on which proxy server be listening.
// It returns address string with no host name and port from env.
func serverAddress() string {
	port := os.Getenv("PORT")
	return ":" + port
}

func main() {
	log.Printf("Starting grproxy server ...")

	// get the host:port value from env and create a url string.
	NginxHostPath = "http://" + os.Getenv("STATIC_CONTENT_HOST")

	// all the request to root url will be handled by proxyHandler. pattern parameter is not using any regex
	http.HandleFunc("/", proxyHandler)

	// start proxy server
	if err := http.ListenAndServe(serverAddress(), nil); err != nil {
		panic(err)
	}
}
