package main

import (
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/antongulenko/golib"
	"github.com/antongulenko/golib/gotermBox"
	"github.com/bitflow-stream/go-bitflow-collector/pcap"
	"github.com/bitflow-stream/go-bitflow-collector/pcap/pcap_impl"
	log "github.com/sirupsen/logrus"
)

const refreshInterval = 500 * time.Millisecond

func main() {
	golib.RegisterFlags(golib.FlagsAll)
	var nics golib.StringSlice
	flag.Var(&nics, "n", "One or more network interfaces to capture packets from")
	flag.Parse()
	golib.ConfigureLogging()
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
	sources, err := pcap_impl.OpenSources("", nics, true)
	golib.Checkerr(err)
	cons.CapturePackets(sources, func(err error) {
		if captureErr, ok := err.(pcap.CaptureError); ok {
			log.Warnln("Capture error:", captureErr)
		} else {
			golib.Fatalln(err)
		}
	})

	task.Update = func(out io.Writer, textWidth int) error {
		noData := 0
		for _, con := range cons.Sorted() {
			if con.HasData() {
				if _, err := fmt.Fprintln(out, con); err != nil {
					return err
				}
			} else {
				noData++
			}
		}
		if noData > 0 {
			if _, err := fmt.Fprintf(out, "\n(+ %v connections without data)\n", noData); err != nil {
				return err
			}
		}
		return nil
	}
	group := golib.TaskGroup{&task, &golib.NoopTask{
		Chan:        golib.ExternalInterrupt(),
		Description: "external interrupt",
	}}
	group.PrintWaitAndStop()
}
