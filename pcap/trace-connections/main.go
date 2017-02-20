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

	task := &golib.LoopTask{
		Description: "print connections",
		Loop: func(stop golib.StopChan) error {
			log.Println("========================================================")
			for _, con := range cons.Sorted() {
				if con.HasData() {
					log.Println(con)
				}
			}
			stop.WaitTimeout(500 * time.Millisecond)
			return nil
		},
	}
	group := golib.TaskGroup{task, &golib.NoopTask{
		Chan:        golib.ExternalInterrupt(),
		Description: "external interrupt",
	}}
	group.PrintWaitAndStop()
}
