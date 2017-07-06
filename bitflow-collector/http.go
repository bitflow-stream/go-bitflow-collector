package main

import (
	"net/http"

	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
)

type FileOutputFilterApi struct {
	lock              sync.Mutex
	FileOutputEnabled bool
}

func (api *FileOutputFilterApi) Register(pathPrefix string, router *mux.Router) {
	router.HandleFunc(pathPrefix+"/file_output", api.handleRequest).Methods("GET", "POST", "PUT", "DELETE")
}

func (api *FileOutputFilterApi) handleRequest(w http.ResponseWriter, r *http.Request) {
	api.lock.Lock()
	oldStatus := api.FileOutputEnabled
	newStatus := oldStatus
	switch r.Method {
	case "GET":
	case "POST", "PUT":
		newStatus = true
	case "DELETE":
		newStatus = false
	}
	api.FileOutputEnabled = newStatus
	api.lock.Unlock()

	var status string
	if api.FileOutputEnabled {
		status = "enabled"
	} else {
		status = "disabled"
	}
	status = "File output is " + status
	if oldStatus != newStatus {
		log.Println(status)
	}
	w.Write([]byte(status + "\n"))
}
