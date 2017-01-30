package main

import (
	"flag"

	"github.com/antongulenko/go-bitflow"
)

var filterAnomalies bool

func init() {
	flag.BoolVar(&filterAnomalies, "store-tagged", false,
		"If true, only write Samples to files as long as tags are defined "+
			"via the REST API (-listen-tags flag). Ignored when -listen-tags is not defined.")
}

type storeAnomalyData struct {
	*bitflow.FileSink
}

func (s *storeAnomalyData) Sample(outSample *bitflow.Sample, header *bitflow.Header) error {
	httpTagsEnabled := currentHttpTags != nil
	if httpTagsEnabled {
		return s.FileSink.Sample(outSample, header)
	}
	return nil
}

func replaceAnomalyFileOutput(sink bitflow.MetricSink) bitflow.MetricSink {
	if !filterAnomalies || taggingPort == 0 {
		return sink
	}

	wrap := func(sink bitflow.MetricSink) bitflow.MetricSink {
		if fileSink, ok := sink.(*bitflow.FileSink); ok {
			return &storeAnomalyData{
				FileSink: fileSink,
			}
		}
		return sink
	}

	if multi, ok := sink.(MultiSink); ok {
		for i, sink := range multi {
			multi[i] = wrap(sink)
		}
		return multi
	} else {
		return wrap(sink)
	}
}
