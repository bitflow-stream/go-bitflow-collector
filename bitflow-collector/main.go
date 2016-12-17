package main

import (
	"flag"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/golib"
)

var (
	print_metrics   = false
	print_graph     = ""
	print_graph_dot = ""
)

func do_main() int {
	// Register and parse command line flags
	flag.BoolVar(&print_metrics, "print-metrics", print_metrics, "Print all available metrics and exit")
	flag.StringVar(&print_graph, "graph", print_graph, "Create png-file for the collector-graph and exit")
	flag.StringVar(&print_graph_dot, "graph-dot", print_graph_dot, "Create dot-file for the collector-graph and exit")

	var f bitflow.EndpointFactory
	f.RegisterGeneralFlagsTo(flag.CommandLine)
	f.RegisterOutputFlagsTo(flag.CommandLine)
	bitflow.RegisterGolibFlags()
	flag.Parse()
	golib.ConfigureLogging()
	if flag.NArg() > 0 {
		log.Fatalln("Stray command line argument(s):", flag.Args())
	}
	registerFlagTags()
	serveTaggingApi()
	sampleTagger.register()
	defer golib.ProfileCpu()()
	if !f.HasOutputFlag() {
		// Make sure to at least output on the console
		f.FlagOutputBox = true
	}

	// Configure and start the data collector
	col := createCollectorSource()
	stop := false
	if print_metrics {
		golib.Checkerr(col.PrintMetrics())
		stop = true
	}
	if print_graph != "" {
		golib.Checkerr(col.PrintGraph(print_graph, all_metrics))
		stop = true
	}
	if print_graph_dot != "" {
		golib.Checkerr(col.PrintGraphDot(print_graph_dot, all_metrics))
		stop = true
	}
	if stop {
		return 0
	}
	p := bitflow.SamplePipeline{
		Source: col,
	}
	if err := p.ConfigureSink(&f); err != nil {
		log.Fatalln(err)
	}
	replaceAnomalyFileOutput(&p)
	return p.StartAndWait()
}

func main() {
	os.Exit(do_main())
}
