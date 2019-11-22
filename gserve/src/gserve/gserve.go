package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

var serverId string

type GetResponse struct {
	Server string `json:"server"`
}

// handleRequests will be on / path and  check which method is called. For POST method it will add data to Hbase
// via Hbase REST API. For GET method it will get all data from Hbase and add it to a html template file with server
// signature line at the bottom.
func handleRequests(res http.ResponseWriter, req *http.Request) {

	switch req.Method {
	case http.MethodGet:
		// TODO: get data from Hbase and populate html file

		// set content type header for response as html
		res.Header().Set("Content-Type", "text/html; charset=utf-8")

		// parse the html file
		t, err := template.ParseFiles(filepath.Join(os.Getenv("TEMPLATE_FOLDER"), "/getTemplate.html"))
		if err != nil {
			log.Println(err)
			_, _ = fmt.Fprintf(res, "Unable to load template")
			return
		}

		// create data for the template
		response := GetResponse{Server: serverId}
		// add data to template and send response.
		err = t.Execute(res, response)
		if err != nil {
			log.Println(err)
		}
		break
	case http.MethodPost:
		// TODO: add data to Hbase
		res.WriteHeader(http.StatusAccepted)
		break
	default:
		res.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// serverAddress reads environment for port on which proxy server be listening.
// It returns address string with no host name and port from env.
func serverAddress() string {
	port := os.Getenv("PORT")
	return ":" + port
}

func main() {
	serverId = os.Getenv("ID")
	log.Printf("Starting gserve instance: %s ...", serverId)

	// all the request to root url will be handled by proxyHandler. pattern parameter is not using any regex
	http.HandleFunc("/", handleRequests)

	// start server
	if err := http.ListenAndServe(serverAddress(), nil); err != nil {
		panic(err)
	}
}
