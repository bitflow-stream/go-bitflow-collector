package main

import (
	"flag"

	"github.com/antongulenko/go-bitflow"
)

var filterAnomalies bool

func init() {
	flag.BoolVar(&filterAnomalies, "store-tagged", false,
		"If true, only write Samples to files (-f) as long as tags are defined "+
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

func replaceAnomalyFileOutput(p *bitflow.SamplePipeline) {
	if !filterAnomalies || taggingPort == 0 {
		return
	}
	if agg, ok := p.Sink.(bitflow.AggregateSink); ok {
		for i, sink := range agg {
			if fileSink, ok := sink.(*bitflow.FileSink); ok {
				agg[i] = &storeAnomalyData{
					FileSink: fileSink,
				}
			}
		}
	}
}
