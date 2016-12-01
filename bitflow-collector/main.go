package main

import (
	"flag"
	"os"

	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/golib"
)

func do_main() int {
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
