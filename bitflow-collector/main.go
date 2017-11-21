package main

import (
	"flag"
	"os"

	"github.com/antongulenko/go-bitflow-collector/cmd_helper"
	"github.com/antongulenko/golib"
	log "github.com/sirupsen/logrus"
)

func main() {
	os.Exit(do_main())
}

func do_main() int {
	print_metrics := flag.Bool("print-metrics", false, "Print all available metrics and exit")
	print_pipeline := flag.Bool("print-pipeline", false, "Print the data collection pipeline and exit")
	print_graph := flag.String("graph", "", "Create png-file for the collector-graph and exit")
	print_graph_dot := flag.String("graph-dot", "", "Create dot-file for the collector-graph and exit")

	// Parse command line flags
	var cmd cmd_helper.CmdDataCollector
	cmd.ParseFlags()
	if flag.NArg() > 0 {
		golib.Fatalln("Stray command line argument(s):", flag.Args())
	}
	defer golib.ProfileCpu()()

	// Configure the data collector pipeline
	collector := createCollectorSource()
	p := cmd.MakePipeline()
	p.Source = collector

	// Print requested information
	stop := false
	if *print_pipeline {
		for _, str := range p.FormatLines() {
			log.Println(str)
		}
		stop = true
	} else {
		for _, str := range p.FormatLines() {
			log.Debugln(str)
		}
	}
	if *print_metrics {
		golib.Checkerr(collector.PrintMetrics())
		stop = true
	}
	if *print_graph != "" {
		golib.Checkerr(collector.PrintGraph(*print_graph, all_metrics))
		stop = true
	}
	if *print_graph_dot != "" {
		golib.Checkerr(collector.PrintGraphDot(*print_graph_dot, all_metrics))
		stop = true
	}
	if stop {
		return 0
	}

	return p.StartAndWait()
}
