package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/samuel/go-zookeeper/zk"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var netClient = http.Client{
	Timeout: time.Second * 5,
}

// Initialize details for gserve host, zookeeper and hbase.
var serverListenAddress string
var serverId string
var port string

var zooKeeperHost string
var hBaseClientAddress string
var hBaseLibraryTable string
var zooKeeperWatchNode = "/gserve"

// struct to hold data for html template, used in get response.
type GetResponse struct {
	Rows   []ResRow
	Server string
}

type ResRow struct {
	Name  string
	Docs  []ResDoc
	Metas []ResMeta
}

type ResMeta struct {
	Name  string
	Value string
}

type ResDoc struct {
	Name  string
	Value string
}

// handleRequests will be on / path and  check which method is called. For POST method it will add data to Hbase
// via Hbase REST API. For GET method it will get all data from Hbase and add it to a html template file with server
// signature line at the bottom.
func handleRequests(res http.ResponseWriter, req *http.Request) {
	log.Println(req.Method, req.URL.Path)
	switch req.Method {
	case http.MethodGet:
		processGetRequest(res, req)
		break
	case http.MethodPost:
		processPostRequest(res, req)
	default:
		log.Println("method not allowed: ", req.Method)
		res.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func processGetRequest(res http.ResponseWriter, req *http.Request) {
	// set content type header for response as html
	res.Header().Set("Content-Type", "text/html; charset=utf-8")

	// create scanner for the library table
	scannerAddress := hBaseLibraryTable + "/scanner"
	request, err := http.NewRequest("PUT", scannerAddress, bytes.NewBuffer([]byte(`<Scanner maxVersions="1"/>`)))
	if err != nil {
		log.Println("error creating scanner request", err)
	}
	// set content type for request body
	request.Header.Set("Content-Type", "text/xml")

	// try the request.
	response, err := netClient.Do(request)
	if err != nil {
		log.Println("error during request", err)
	}

	// read scanner url from response header.
	scanner := response.Header.Get("Location")
	//log.Println(response.Header, scanner)

	// create request for scanner url
	request, err = http.NewRequest("GET", scanner, nil)
	if err != nil {
		log.Println("error creating read scanner request", err)
	}
	// set content type we need from scanner
	request.Header.Set("Accept", "application/json")

	response, err = netClient.Do(request)
	if err != nil {
		log.Println("scanner request error", err)
	}

	var encodedRowData EncRowsType
	// decode the response body to json and unmarshal it to encoded row type
	err = json.NewDecoder(response.Body).Decode(&encodedRowData)
	if err != nil {
		log.Println("scanner read json decoding error: ", err)
	}
	// decode the result to decode base64 back to normal string
	rowData, err := encodedRowData.decode()
	if err != nil {
		log.Println("base64 decoding error", err)
	}
	//log.Println(rowData)

	// convert the scanner results to data format which will be used in html template.
	var rows []ResRow
	for _, row := range rowData.Row {

		resRow := ResRow{
			Name: row.Key,
		}
		for _, cell := range row.Cell {
			if strings.HasPrefix(cell.Column, "document") {
				doc := ResDoc{
					Name:  strings.Split(cell.Column, ":")[1],
					Value: cell.Value,
				}
				resRow.Docs = append(resRow.Docs, doc)
			} else {
				meta := ResMeta{
					Name:  strings.Split(cell.Column, ":")[1],
					Value: cell.Value,
				}
				resRow.Metas = append(resRow.Metas, meta)
			}
		}

		rows = append(rows, resRow)
	}

	// create data for the template
	result := GetResponse{Server: serverId, Rows: rows}

	// parse the html file
	t, err := template.ParseFiles(filepath.Join(os.Getenv("TEMPLATE_FOLDER"), "/getTemplate.html"))
	if err != nil {
		log.Println(err)
		_, _ = fmt.Fprintf(res, "Unable to load template")
		return
	}

	// add data to template and send response.
	err = t.Execute(res, result)
	if err != nil {
		log.Println(err)
	}
}

func processPostRequest(res http.ResponseWriter, req *http.Request) {

	// create url to send data to hbase
	now := time.Now()
	timestamp := uint64(now.Unix())
	newRowAddress := hBaseLibraryTable + "/" + strconv.FormatUint(timestamp, 10)

	var rowData RowsType

	// assuming that correct data is sent in request convert it to RowsType
	err := json.NewDecoder(req.Body).Decode(&rowData)
	if err != nil {
		log.Println(err)
		res.WriteHeader(http.StatusBadRequest)
		return
	}
	// encode the values to base64 for sending to hbase
	encodedRowData := rowData.encode()

	// Marshal the data to json
	requestBody, err := json.Marshal(encodedRowData)
	if err != nil {
		log.Println(err)
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	// create post request to hbase instance for storing this data.
	request, err := http.NewRequest("PUT", newRowAddress, bytes.NewBuffer(requestBody))
	if err != nil {
		log.Println(err)
		res.WriteHeader(http.StatusInternalServerError)
		return
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	// send request to hbase
	response, err := netClient.Do(request)

	if err != nil {
		log.Println(err)
		res.WriteHeader(http.StatusInternalServerError)
		return
	} else {
		log.Println("hbase put data result: ", response.Status, response.StatusCode)
		res.WriteHeader(http.StatusAccepted)
	}

}

// connectWithOptions will try to connect with zookeeper instance.
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

// waitForGServeWatchNode will wait until node in zookeeper is created (event received). This will ensure that server
// waits for zookeeper to publish its url and then start http server
func waitForGServeWatchNode(conn *zk.Conn) {
	present, _, events, err := conn.ExistsW(zooKeeperWatchNode)

	if err != nil {
		log.Println("error exists check: ", err)
	}

	if present {
		return
	}
	for {
		select {
		case event := <-events:
			if event.Type == zk.EventNodeCreated {
				log.Printf(event.Path, event)
				return
			}

		}
	}
}

// publishServerDetails sends the details to zookeeper for the current instance. It will first try to create /gserve
// parent node, then publish the host url against id of this server under /gserve.
func publishServerDetails(conn *zk.Conn, myUrl string) {
	log.Println("publishing gserve host details to zookeeper")

	var publishNode = zooKeeperWatchNode + "/" + serverId
	_, err := conn.Create(publishNode, []byte(myUrl), zk.FlagEphemeral, zk.WorldACL(zk.PermRead))

	if err != nil {
		log.Println("error publishing connection", err)
	} else {
		log.Println("host details published to " + publishNode)
	}

}

// getEnvironment tries to read environment otherwise sets default values.
func getEnvironment() {
	envVal, present := os.LookupEnv("ZOOKEEPER_HOST")
	if present {
		zooKeeperHost = envVal
	} else {
		zooKeeperHost = "zookeeper"
	}

	envVal, present = os.LookupEnv("ID")
	if present {
		serverId = envVal
	} else {
		serverId = "gserve"
	}

	envVal, present = os.LookupEnv("PORT")
	if present {
		port = envVal
		serverListenAddress = ":" + envVal
	} else {
		port = "80"
		serverListenAddress = ":80"
	}

	envVal, present = os.LookupEnv("HBASE_HOST")
	if present {
		hBaseClientAddress = "http://" + envVal
		hBaseLibraryTable = hBaseClientAddress + "/se2:library"
	} else {
		hBaseClientAddress = "http://hbase:8080"
		hBaseLibraryTable = hBaseClientAddress + "/se2:library"
	}
}

func main() {
	log.Printf("Starting gserve instance: %s ...", serverId)

	getEnvironment()

	// prepare url for this instance and publish it.
	myUrl := "http://" + serverId + ":" + port
	conn, _ := connectWithOptions()
	waitForGServeWatchNode(conn)
	publishServerDetails(conn, myUrl)

	// all the request to root url will be handled by proxyHandler. pattern parameter is not using any regex
	http.HandleFunc("/", handleRequests)

	// start http server
	if err := http.ListenAndServe(serverListenAddress, nil); err != nil {
		panic(err)
	}
}
