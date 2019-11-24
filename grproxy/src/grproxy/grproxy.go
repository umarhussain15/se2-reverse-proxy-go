package main

import (
	"github.com/samuel/go-zookeeper/zk"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// NginxHostPath will hold the url of nginx instance. e.g http://nginx
var NginxHostPath string

var zooKeeperHost = os.Getenv("ZOOKEEPER_HOST")
var zooKeeperReadNode = "/gserve"
var gServeAddresses []string

var currentServerIndex = -1

var wg sync.WaitGroup

// proxyHandler will call either dynamicContentProxy or staticContentProxy based on the prefix "/library" presence
// in the zooKeeperReadNode of the request url.
func proxyHandler(res http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	if strings.HasPrefix(path, "/library") {
		dynamicContentProxy(res, req)
	} else {
		staticContentProxy(res, req)
	}
}

func dynamicContentProxy(res http.ResponseWriter, req *http.Request) {
	//log.Printf("dynamic request handler %s", req.URL.Path)
	var serverAddress string
	if len(gServeAddresses) != 0 {
		wg.Wait()
		wg.Add(1)
		if currentServerIndex+1 == len(gServeAddresses) {
			currentServerIndex = 0
		} else {
			currentServerIndex++
		}
		serverAddress = gServeAddresses[currentServerIndex]
		wg.Done()
	} else {
		log.Println("No server address present")
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	serverUrl, _ := url.Parse(serverAddress)
	httputil.NewSingleHostReverseProxy(serverUrl).ServeHTTP(res, req)
}

// staticContentProxy will use reverse proxy with host url of our nginx instance. nginx will respond to the request and
// go will send back the result it received from nginx.
func staticContentProxy(res http.ResponseWriter, req *http.Request) {
	//log.Printf("proxying to nginx for: %s", req.URL.Path)
	serverUrl, _ := url.Parse(NginxHostPath)
	httputil.NewSingleHostReverseProxy(serverUrl).ServeHTTP(res, req)
}

func connectWithOptions() (*zk.Conn, <-chan zk.Event) {
	log.Print("establishing connection to zookeeper with options")
	conn, events, err := zk.Connect([]string{zooKeeperHost}, time.Second*20)
	if err != nil {
		panic(err)
	}
	for event := range events {
		if event.State == zk.StateHasSession {
			return conn, events
		}
	}
	return conn, events
}

func createGServeWatchNode(conn *zk.Conn) {

	_, err := conn.Create(zooKeeperReadNode, []byte{}, 0, zk.WorldACL(zk.PermAll))

	if err != nil {
		log.Println("error creating /gserve", err)
	} else {
		log.Println("created /gserve node in zookeeper")
	}
	//watchChildren(conn)
}

func watchGServeNodeChildren(conn *zk.Conn) {
	nodes, _, events, err := conn.ChildrenW(zooKeeperReadNode)
	if err != nil {
		log.Println(err)
	} else {
		log.Print(nodes)
		getNodesData(conn, nodes)
		log.Print("receive child events")
		for {
			select {
			case event := <-events:
				if event.Type == zk.EventNodeChildrenChanged {
					log.Printf(event.Path, event)
					go watchGServeNodeChildren(conn)
					return
				}
			}
		}
	}
}

func getNodesData(conn *zk.Conn, nodes []string) {
	var tempAddress []string
	for _, node := range nodes {
		if data, _, err := conn.Get(zooKeeperReadNode + "/" + node); err != nil {
			log.Printf("cannot read node data: %v", node)
		} else {
			tempAddress = append(tempAddress, string(data))
			log.Printf("node data: %v", string(data))
		}
	}
	gServeAddresses = tempAddress
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

	conn, _ := connectWithOptions()
	createGServeWatchNode(conn)

	// watch in separate routine
	go watchGServeNodeChildren(conn)
	// all the request to root url will be handled by proxyHandler. pattern parameter is not using any regex
	http.HandleFunc("/", proxyHandler)

	// start proxy server
	if err := http.ListenAndServe(serverAddress(), nil); err != nil {
		panic(err)
	}
}
