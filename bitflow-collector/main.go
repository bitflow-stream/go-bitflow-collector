package main

import (
	"flag"
	"fmt"
	"os"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/golib"
)

func main() {
	os.Exit(do_main())
}

func do_main() int {
	// Register and parse command line flags
	print_metrics := flag.Bool("print-metrics", false, "Print all available metrics and exit")
	print_graph := flag.String("graph", "", "Create png-file for the collector-graph and exit")
	print_graph_dot := flag.String("graph-dot", "", "Create dot-file for the collector-graph and exit")
	var sink_flags golib.StringSlice
	flag.Var(&sink_flags, "o", "Data sink(s) for outputting data")

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

	// Configure and start the data collector
	col := createCollectorSource()
	stop := false
	if *print_metrics {
		golib.Checkerr(col.PrintMetrics())
		stop = true
	}
	if *print_graph != "" {
		golib.Checkerr(col.PrintGraph(*print_graph, all_metrics))
		stop = true
	}
	if *print_graph_dot != "" {
		golib.Checkerr(col.PrintGraphDot(*print_graph_dot, all_metrics))
		stop = true
	}
	if stop {
		return 0
	}
	p := bitflow.SamplePipeline{
		Source: col,
		Sink:   create_output(&f, sink_flags),
	}
	p.Sink = replaceAnomalyFileOutput(p.Sink)
	return p.StartAndWait()
}

func create_output(f *bitflow.EndpointFactory, outputs []string) bitflow.MetricSink {
	if len(outputs) == 0 {
		outputs = []string{"box://-"} // Print to console as default
	}
	var sinks MultiSink
	consoleOutput := false
	for _, output := range outputs {
		sink, err := f.CreateOutput(output)
		sinks = append(sinks, sink)
		golib.Checkerr(err)
		newConsoleOutput := bitflow.IsConsoleOutput(sink)
		if consoleOutput && newConsoleOutput {
			log.Fatalln("Cannot define multiple outputs to stdout")
		}
		consoleOutput = newConsoleOutput
	}
	if len(sinks) == 1 {
		return sinks[0]
	} else {
		return sinks
	}
}

type MultiSink []bitflow.MetricSink

func (s MultiSink) Sample(sample *bitflow.Sample, header *bitflow.Header) error {
	var err golib.MultiError
	for _, out := range s {
		err.Add(out.Sample(sample, header))
	}
	return err.NilOrError()
}

func (s MultiSink) Start(wg *sync.WaitGroup) golib.StopChan {
	chans := make([]golib.StopChan, 0, len(s))
	for _, out := range s {
		if c := out.Start(wg); c != nil {
			chans = append(chans, c)
		}
	}
	if len(chans) == 0 {
		return nil
	}

	stopper := make(chan error, 2)
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := golib.WaitForAny(chans)
		stopper <- err
	}()
	return stopper
}

func (s MultiSink) Close() {
	for _, out := range s {
		out.Close()
	}
}

func (s MultiSink) Stop() {
}

func (s MultiSink) String() string {
	return fmt.Sprintf("MultiSink (len %v)", len(s))
}
