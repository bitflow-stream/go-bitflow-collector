package main

import (
	"flag"
	"io"
	"math"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/antongulenko/go-bitflow-collector/pcap"
	"github.com/antongulenko/golib"
)

const snaplen = math.MaxInt32

func main() {
	golib.RegisterLogFlags()
	flag.Parse()
	defer golib.ProfileCpu()()
	traceConnections("wlan0")
}

func traceConnections(nic string) {
	cons := pcap.NewConnections()
	source, err := pcap.OpenPcap(nic, snaplen)
	if err != nil {
		log.Fatalln(err)
	}

	go func() {
		for {
			err := pcap.CaptureOnePacket(source, cons)
			if err != nil {
				if err == io.EOF {
					break
				} else if captureErr, ok := err.(pcap.CaptureError); ok {
					log.Warnln("Capture error:", captureErr)
				} else {
					log.Fatalln(err)
				}
			}
		}
	}()

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
