package main

import (
	"bytes"
	"flag"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/antongulenko/go-bitflow"
)

const (
	form_timeout = "timeout"
)

var (
	currentHttpTags map[string]string

	taggingPort    int
	taggingTimeout time.Time
	loopCond       = sync.NewCond(new(sync.Mutex))
)

func init() {
	flag.IntVar(&taggingPort, "listen-tags", 0, "Enable tagging HTTP API on the given port. "+
		"Samples will carry the defined tags until the timeout expires. Tags can be arbitrary, empty list is allowed. "+
		"POST: /tag?timeout=<SECONDS>&<TAG1>=<VAL1>&<TAG2>=<VAL2>&... ")
}

func serveTaggingApi() {
	if taggingPort == 0 {
		return
	}
	sampleTagger.taggerFuncs = append(sampleTagger.taggerFuncs, httpTagger)
	http.HandleFunc("/tag", handleTaggingRequest)
	go checkTimeoutLoop()
	go func() {
		log.Fatalln(http.ListenAndServe(":"+strconv.Itoa(taggingPort), nil))
	}()
}

func handleTaggingRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(405)
		w.Write([]byte("Only POST method is allowed, not " + r.Method))
		return
	}
	timeout := r.FormValue(form_timeout)
	if timeout == "" {
		w.WriteHeader(400)
		w.Write([]byte(fmt.Sprintf("Need URL parameter %s", form_timeout)))
		return
	}
	timeoutSec, err := strconv.Atoi(timeout)
	if err != nil {
		w.WriteHeader(400)
		w.Write([]byte(fmt.Sprintf("Failed to parse %s parameter: %v", form_timeout, err)))
		return
	}
	newTags := make(map[string]string)
	for key, val := range r.Form {
		if key != form_timeout && len(val) > 0 {
			newTags[key] = val[0]
		}
	}

	loopCond.L.Lock()
	defer loopCond.L.Unlock()
	log.Println("Setting tags to", newTags, "previously:", currentHttpTags)
	currentHttpTags = newTags
	taggingDuration := time.Duration(timeoutSec) * time.Second
	taggingTimeout = time.Now().Add(taggingDuration)
	loopCond.Broadcast()

	w.WriteHeader(200)
	var content string
	if taggingDuration == 0 {
		content = "Unsetting tags"
	} else {
		tagStr := "No tags set"
		if len(currentHttpTags) > 0 {
			tagStr = "Tags set to " + printMap(currentHttpTags)
		}
		content = fmt.Sprintf("%v for %v (until %v)", tagStr, taggingDuration, taggingTimeout.Format("2006-01-02 15:04:05.000"))
	}
	w.Write([]byte(content + "\n"))
}

func printMap(values map[string]string) string {
	var buf bytes.Buffer
	started := false
	for key, val := range values {
		if started {
			buf.WriteString(", ")
		}
		buf.WriteString(key)
		buf.WriteString("=")
		buf.WriteString(val)
		started = true
	}
	return buf.String()
}

func checkTimeoutLoop() {
	loopCond.L.Lock()
	defer loopCond.L.Unlock()
	for {
		t := taggingTimeout.Sub(time.Now())
		if t <= 0 {
			if currentHttpTags != nil {
				log.Println("Timeout: Unsetting tags, previously:", currentHttpTags)
				currentHttpTags = nil
			}
		} else {
			time.AfterFunc(t, func() {
				loopCond.L.Lock()
				defer loopCond.L.Unlock()
				loopCond.Broadcast()
			})
		}
		loopCond.Wait()
	}
}

func httpTagger(s *bitflow.Sample) {
	if current := currentHttpTags; current != nil {
		for tag, val := range current {
			s.SetTag(tag, val)
		}
	}
}
