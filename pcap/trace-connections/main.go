package main

import (
	"flag"
	"math"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/go-bitflow-collector/pcap"
	"github.com/antongulenko/golib"
)

const snaplen = math.MaxInt32

func main() {
	bitflow.RegisterGolibFlags()
	flag.Parse()
	defer golib.ProfileCpu()()
	traceConnections("wlan0")
}

func traceConnections(nics ...string) {
	cons := pcap.NewConnections()
	err := cons.CaptureNics(nics, snaplen, func(err error) {
		if captureErr, ok := err.(pcap.CaptureError); ok {
			log.Warnln("Capture error:", captureErr)
		} else {
			log.Fatalln(err)
		}
	})
	if err != nil {
		log.Fatalln(err)
	}

	task := golib.NewLoopTask("print connections", func(stop golib.StopChan) {
		log.Println("========================================================")
		for _, con := range cons.Sorted() {
			if con.HasData() {
				log.Println(con)
			}
		}
		select {
		case <-time.After(500 * time.Millisecond):
		case <-stop:
		}
	})
	golib.NewTaskGroup(task, &golib.NoopTask{
		Chan:        golib.ExternalInterrupt(),
		Description: "external interrupt",
	}).PrintWaitAndStop()
}
