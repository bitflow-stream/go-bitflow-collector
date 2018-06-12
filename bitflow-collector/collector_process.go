package main

import (
	"bytes"
	"flag"
	"fmt"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/antongulenko/go-bitflow-collector"
	"github.com/antongulenko/go-bitflow-collector/psutil"
	"github.com/antongulenko/go-bitflow-pipeline/collector_helpers"
	"github.com/antongulenko/golib"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

var proc_update_pids time.Duration

func init() {
	flag.DurationVar(&proc_update_pids, "proc-interval", 1500*time.Millisecond, "Interval for updating list of observed pids")
}

func createProcessCollectors(cmd *collector_helpers.CmdDataCollector) []collector.Collector {
	psutilRoot := psutil.NewPsutilRootCollector(&ringFactory)
	psutilProcesses := psutilRoot.NewMultiProcessCollector("processes")
	multiProcs := &MonitorProcessesRestApi{
		procs: psutilProcesses,
	}
	if err := multiProcs.updateCollectors(); err != nil {
		golib.Checkerr(err)
	}
	cmd.RestApis = append(cmd.RestApis, multiProcs)
	psutil.PidUpdateInterval = proc_update_pids
	return []collector.Collector{psutilRoot, psutilProcesses}
}

type MonitorProcessesRestApi struct {
	procs *psutil.PsutilMultiProcessCollector
	lock  sync.Mutex

	proc_collectors          golib.KeyValueStringSlice
	proc_children_collectors golib.KeyValueStringSlice
	proc_show_errors         bool
}

func (api *MonitorProcessesRestApi) RegisterFlags() {
	flag.Var(&api.proc_collectors, "proc", "'key=regex' Processes to collect metrics for (regex match on entire command line)")
	flag.Var(&api.proc_children_collectors, "proc-children", "'key=regex' Processes to collect metrics for (regex match on entire command line). Include all child processes of matched processes.")
	flag.BoolVar(&api.proc_show_errors, "proc-show-errors", false, "Verbose: show errors encountered while getting process metrics")
}

func (api *MonitorProcessesRestApi) Register(pathPrefix string, router *mux.Router) {
	router.HandleFunc(pathPrefix+"/proc", api.handleProcRootRequest).Methods("GET", "DELETE")
	router.HandleFunc(pathPrefix+"/proc-children", api.handleProcChildrenRootRequest).Methods("GET", "DELETE")
	router.HandleFunc(pathPrefix+"/proc/{name}", api.handleProcRequest).Methods("GET", "POST", "PUT", "DELETE")
	router.HandleFunc(pathPrefix+"/proc-children/{name}", api.handleProcChildrenRequest).Methods("GET", "POST", "PUT", "DELETE")
}

func (api *MonitorProcessesRestApi) updateCollectors() error {
	desc1, err := api.createCollectors(api.proc_collectors, false)
	if err != nil {
		return err
	}
	desc2, err := api.createCollectors(api.proc_children_collectors, true)
	if err != nil {
		return err
	}
	api.procs.Processes = append(desc1, desc2...)
	api.procs.UpdateProcesses()
	return nil
}

func (api *MonitorProcessesRestApi) createCollectors(parameters golib.KeyValueStringSlice, includeChildren bool) ([]psutil.PsutilProcessCollectorDescription, error) {
	res := make([]psutil.PsutilProcessCollectorDescription, 0, len(parameters.Keys))
	if len(parameters.Keys) > 0 {
		regexes := make(map[string][]*regexp.Regexp)
		for key, value := range parameters.Map() {
			regex, err := regexp.Compile(value)
			if err != nil {
				return nil, fmt.Errorf("Error compiling regex '%v' for process group '%v': %v", value, key, err)
			}
			regexes[key] = append(regexes[key], regex)
		}
		for key, list := range regexes {
			desc := psutil.PsutilProcessCollectorDescription{key, list, api.proc_show_errors, includeChildren}
			res = append(res, desc)
		}
	}
	return res, nil
}

func (api *MonitorProcessesRestApi) writeStatus(w http.ResponseWriter, r *http.Request) {
	var out bytes.Buffer
	api.printProcesses("Monitored processes", &out, &api.proc_collectors)
	api.printProcesses("Monitored process groups (including recursive children)", &out, &api.proc_children_collectors)
	w.Write(out.Bytes())
}

func (api *MonitorProcessesRestApi) printProcesses(name string, out *bytes.Buffer, procs *golib.KeyValueStringSlice) {
	if len(procs.Keys) == 0 {
		out.WriteString("No " + name + "\n")
	} else {
		out.WriteString(name + ":\n")
		for name, regex := range procs.Map() {
			out.WriteString(name)
			out.WriteString(" -> ")
			out.WriteString(regex)
			out.WriteString("\n")
		}
	}
}

func (api *MonitorProcessesRestApi) update(w http.ResponseWriter, r *http.Request) {
	err := api.updateCollectors()
	if err != nil {
		w.Write([]byte("Error: " + err.Error() + "\n"))
		log.Errorln("Error updating monitored processes:", err)
		return
	}
	api.writeStatus(w, r)
}

func (api *MonitorProcessesRestApi) handleProcRootRequest(w http.ResponseWriter, r *http.Request) {
	api.handleRootRequest("individual processes", w, r, &api.proc_collectors)
}

func (api *MonitorProcessesRestApi) handleProcChildrenRootRequest(w http.ResponseWriter, r *http.Request) {
	api.handleRootRequest("recursive process groups", w, r, &api.proc_children_collectors)
}

func (api *MonitorProcessesRestApi) handleProcRequest(w http.ResponseWriter, r *http.Request) {
	api.handleIndividualRequest("individual process", w, r, &api.proc_collectors)
}

func (api *MonitorProcessesRestApi) handleProcChildrenRequest(w http.ResponseWriter, r *http.Request) {
	api.handleIndividualRequest("recursive process group", w, r, &api.proc_children_collectors)
}

func (api *MonitorProcessesRestApi) handleRootRequest(description string, w http.ResponseWriter, r *http.Request, slice *golib.KeyValueStringSlice) {
	api.lock.Lock()
	defer api.lock.Unlock()

	switch r.Method {
	case "GET":
		api.writeStatus(w, r)
	case "DELETE":
		log.Println("Stopped monitoring all " + description)
		slice.Keys = slice.Keys[0:0]
		slice.Values = slice.Values[0:0]
		api.update(w, r)
	}
}

func (api *MonitorProcessesRestApi) handleIndividualRequest(description string, w http.ResponseWriter, r *http.Request, slice *golib.KeyValueStringSlice) {
	api.lock.Lock()
	defer api.lock.Unlock()

	switch r.Method {
	case "GET":
		api.writeStatus(w, r)
	case "POST", "PUT":
		name := mux.Vars(r)["name"]
		regexStr := r.FormValue("regex")
		if regexStr == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Missing URL parameter 'regex'\n"))
			return
		}
		log.Printf("Monitoring %v '%v': %v", description, name, regexStr)
		slice.Put(name, regexStr)
		api.update(w, r)
	case "DELETE":
		name := mux.Vars(r)["name"]
		log.Printf("Stopped monitoring %v '%v'", description, name)
		slice.Delete(name)
		api.update(w, r)
	}
}
