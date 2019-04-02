package main

import (
	"flag"
	"os"
	"strings"

	"github.com/antongulenko/golib"
	"github.com/bitflow-stream/go-bitflow/cmd"
	log "github.com/sirupsen/logrus"
)

func main() {
	os.Exit(do_main())
}

func do_main() int {
	print_metrics := flag.Bool("print-metrics", false, "Print all available metrics and exit")
	print_root_collectors := flag.Bool("print-root-collectors", false, "Print the available root collectors and exit")
	print_graph := flag.String("graph", "", "Create png-file for the collector-graph and exit")
	print_graph_dot := flag.String("graph-dot", "", "Create dot-file for the collector-graph and exit")

	// Parse command line flags
	helper := cmd.CmdDataCollector{DefaultOutput: "box://-"}
	helper.RegisterFlags()
	_, args := cmd.ParseFlags()
	if len(args) > 0 {
		golib.Fatalln("Stray command line argument(s):", args)
	}
	defer golib.ProfileCpu()()

	// Configure the data collector pipeline
	collector := createCollectorSource(&helper)
	p, err := helper.BuildPipeline()
	golib.Checkerr(err)
	if p == nil {
		return 0
	}
	p.Source = collector

	// Print requested information
	stop := false
	if *print_root_collectors {
		rootNames := make([]string, len(collector.RootCollectors))
		for i, col := range collector.RootCollectors {
			rootNames[i] = col.String()
		}
		log.Println("Root collectors:", strings.Join(rootNames, ", "))
		stop = true
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
