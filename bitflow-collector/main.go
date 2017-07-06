package main

import (
	"flag"
	"os"

	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/go-bitflow-pipeline"
	"github.com/antongulenko/go-bitflow-pipeline/fork"
	"github.com/antongulenko/go-bitflow-pipeline/http_tags"
	"github.com/antongulenko/golib"
	"github.com/gorilla/mux"
)

func main() {
	os.Exit(do_main())
}

const restApiPathPrefix = "/api"

var (
	restApiEndpoint string
	fileOutputApi   FileOutputFilterApi
)

func do_main() int {
	print_metrics := flag.Bool("print-metrics", false, "Print all available metrics and exit")
	print_pipeline := flag.Bool("print-pipeline", false, "Print the data collection pipeline and exit")
	print_graph := flag.String("graph", "", "Create png-file for the collector-graph and exit")
	print_graph_dot := flag.String("graph-dot", "", "Create dot-file for the collector-graph and exit")
	var sink_flags golib.StringSlice
	flag.Var(&sink_flags, "o", "Data sink(s) for outputting data")
	var flagTags golib.KeyValueStringSlice
	flag.Var(&flagTags, "tag", "All collected samples will have the given tags (key=value) attached.")
	flag.StringVar(&restApiEndpoint, "api", "", "Enable REST API for controlling the collector. "+
		"The API can be used to control tags and enable/disable file output.")

	// Parse command line flags
	f := bitflow.NewEndpointFactory()
	f.RegisterGeneralFlagsTo(flag.CommandLine)
	f.RegisterOutputFlagsTo(flag.CommandLine)
	bitflow.RegisterGolibFlags()
	flag.Parse()
	golib.ConfigureLogging()
	if flag.NArg() > 0 {
		golib.Fatalln("Stray command line argument(s):", flag.Args())
	}
	defer golib.ProfileCpu()()

	// Configure the data collector pipeline
	collector := createCollectorSource()
	var p pipeline.SamplePipeline
	p.Source = collector
	if len(flagTags.Keys) > 0 {
		p.Add(pipeline.NewTaggingProcessor(flagTags.Map()))
	}
	if restApiEndpoint != "" {
		router := mux.NewRouter()
		tagger := http_tags.NewHttpTagger(restApiPathPrefix, router)
		fileOutputApi.Register(restApiPathPrefix, router)
		server := http.Server{
			Addr:    restApiEndpoint,
			Handler: router,
		}
		// Do not add this routine to any wait group, as it cannot be stopped
		go func() {
			tagger.Error(server.ListenAndServe())
		}()
		p.Add(tagger)
	}
	add_outputs(&p, f, sink_flags)

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

func add_outputs(p *pipeline.SamplePipeline, f *bitflow.EndpointFactory, outputStrings []string) {
	outputs := create_outputs(f, outputStrings)
	if len(outputs) == 1 {
		set_sink(p, outputs[0])
	} else {
		p.Sink = new(bitflow.EmptyMetricSink)

		// Create a multiplex-fork for all outputs
		num := len(outputs)
		builder := make(fork.MultiplexPipelineBuilder, num)
		for i, sink := range outputs {
			builder[i] = new(pipeline.SamplePipeline)
			set_sink(builder[i], sink)
		}
		p.Add(&fork.MetricFork{
			ParallelClose: true,
			Distributor:   fork.NewMultiplexDistributor(num),
			Builder:       builder,
		})
	}
}

func create_outputs(f *bitflow.EndpointFactory, outputs []string) []bitflow.MetricSink {
	if len(outputs) == 0 {
		outputs = []string{"box://-"} // Print to console as default
	}
	var sinks []bitflow.MetricSink
	consoleOutputs := 0
	for _, output := range outputs {
		sink, err := f.CreateOutput(output)
		sinks = append(sinks, sink)
		golib.Checkerr(err)
		if bitflow.IsConsoleOutput(sink) {
			consoleOutputs++
		}
		if consoleOutputs > 1 {
			golib.Fatalln("Cannot define multiple outputs to stdout")
		}
	}
	return sinks
}

func set_sink(p *pipeline.SamplePipeline, sink bitflow.MetricSink) {
	p.Sink = sink

	// Add a filter to file outputs
	if _, isFile := sink.(*bitflow.FileSink); isFile {
		if restApiEndpoint != "" {
			p.Add(&pipeline.SampleFilter{
				Description: pipeline.String("Filter samples while no tags are defined via REST"),
				IncludeFilter: func(sample *bitflow.Sample, header *bitflow.Header) (bool, error) {
					return fileOutputApi.FileOutputEnabled, nil
				},
			})
		}
	}
}
