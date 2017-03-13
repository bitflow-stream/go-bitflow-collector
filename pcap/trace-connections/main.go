package main

import (
	"flag"
	"fmt"
	"io"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/go-bitflow-collector/pcap"
	"github.com/antongulenko/golib"
	"github.com/antongulenko/golib/gotermBox"
)

const (
	snaplen         = int32(65535)
	refreshInterval = 500 * time.Millisecond
)

func main() {
	bitflow.RegisterGolibFlags()
	var nics golib.StringSlice
	flag.Var(&nics, "n", "One or more network interfaces to capture packets from")
	flag.Parse()
	golib.ConfigureLogging()
	if len(nics) == 0 {
		golib.Fatalln("Please provide at least one -n <NIC> parameter")
	}
	defer golib.ProfileCpu()()
	traceConnections(nics...)
}

func traceConnections(nics ...string) {
	task := gotermBox.CliLogBoxTask{
		UpdateInterval: refreshInterval,
		CliLogBox: gotermBox.CliLogBox{
			NoUtf8:        false,
			LogLines:      10,
			MessageBuffer: 1000,
		},
	}
	task.Init()

	cons := pcap.NewConnections()
	err := cons.CaptureNics(nics, snaplen, func(err error) {
		if captureErr, ok := err.(pcap.CaptureError); ok {
			log.Warnln("Capture error:", captureErr)
		} else {
			golib.Fatalln(err)
		}
	})
	if err != nil {
		golib.Fatalln(err)
	}

	task.Update = func(out io.Writer, textWidth int) error {
		noData := 0
		for _, con := range cons.Sorted() {
			if con.HasData() {
				fmt.Fprintln(out, con)
			} else {
				noData++
			}
		}
		if noData > 0 {
			fmt.Fprintf(out, "\n(+ %v connections without data)\n", noData)
		}
		return nil
	}
	group := golib.TaskGroup{&task, &golib.NoopTask{
		Chan:        golib.ExternalInterrupt(),
		Description: "external interrupt",
	}}
	group.PrintWaitAndStop()
}
