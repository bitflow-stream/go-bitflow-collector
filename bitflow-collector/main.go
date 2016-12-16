package main

import (
	"flag"
	"log"
	"os"

	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/golib"
)

func do_main() int {
	// Register and parse command line flags
	var p bitflow.CmdSamplePipeline
	p.RegisterFlags(map[string]bool{
		// Only local samples will be generated.
		"robust": true, "Llimit": true,
	})
	golib.RegisterLogFlags()
	flag.Parse()
	golib.ConfigureLogging()
	if flag.NArg() > 0 {
		log.Fatalln("Stray command line argument(s):", flag.Args())
	}
	registerFlagTags()
	serveTaggingApi()
	sampleTagger.register()
	defer golib.ProfileCpu()()
	if !p.HasOutputFlag() {
		// Make sure to at least output on the console
		p.FlagOutputBox = true
	}

	// Configure and start the data collector
	configurePcap()
	col := createCollectorSource()
	if print_metrics {
		col.PrintMetrics()
		return 0
	}
	p.Source = col
	p.Init()
	replaceAnomalyFileOutput(&p)
	return p.StartAndWait()
}

func main() {
	os.Exit(do_main())
}
