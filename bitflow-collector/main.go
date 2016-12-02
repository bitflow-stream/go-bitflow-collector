package main

import (
	"flag"
	"os"

	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/golib"
)

var (
	print_metrics   = false
	print_graph     = ""
	print_graph_dot = ""
)

func do_main() int {
	flag.BoolVar(&print_metrics, "metrics", print_metrics, "Print all available metrics and exit")
	flag.StringVar(&print_graph, "graph", print_graph, "Create png-file for the collector-graph and exit")
	flag.StringVar(&print_graph_dot, "graph-dot", print_graph_dot, "Create dot-file for the collector-graph and exit")

	// Register and parse command line flags
	var p bitflow.CmdSamplePipeline
	p.RegisterFlags(map[string]bool{
		// Suppress configuring the data input. Only local samples will be generated
		"i": true, "C": true, "F": true, "L": true, "D": true, "FR": true, "robust": true,
	})
	golib.RegisterLogFlags()
	flag.Parse()
	registerFlagTags()
	serveTaggingApi()
	sampleTagger.register()
	golib.ConfigureLogging()
	defer golib.ProfileCpu()()
	if !p.HasOutputFlag() {
		// Make sure to at least output on the console
		p.FlagOutputBox = true
	}

	// Configure and start the data collector
	col := createCollectorSource()
	stop := false
	if print_metrics {
		golib.Checkerr(col.PrintMetrics())
		stop = true
	}
	if print_graph != "" {
		golib.Checkerr(col.PrintGraph(print_graph))
		stop = true
	}
	if print_graph_dot != "" {
		golib.Checkerr(col.PrintGraphDot(print_graph_dot))
		stop = true
	}
	if stop {
		return 0
	}
	p.SetSource(col)
	p.Init()
	replaceAnomalyFileOutput(&p)
	return p.StartAndWait()
}

func main() {
	os.Exit(do_main())
}
