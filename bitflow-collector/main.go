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
	var f bitflow.EndpointFactory
	f.RegisterGeneralFlagsTo(flag.CommandLine)
	f.RegisterOutputFlagsTo(flag.CommandLine)
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
	if !f.HasOutputFlag() {
		// Make sure to at least output on the console
		f.FlagOutputBox = true
	}

	// Configure and start the data collector
	configurePcap()
	col := createCollectorSource()
	if print_metrics {
		col.PrintMetrics()
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
