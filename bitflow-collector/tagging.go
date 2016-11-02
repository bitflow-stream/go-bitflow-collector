package main

import (
	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/go-bitflow-collector"
)

var sampleTagger = new(collectedSampleHandler)

type collectedSampleHandler struct {
	taggerFuncs []func(sample *bitflow.Sample)
}

func (tagger *collectedSampleHandler) register() {
	if len(tagger.taggerFuncs) == 0 {
		return
	}
	collector.CollectedSampleHandler = tagger
}

func (*collectedSampleHandler) HandleHeader(header *bitflow.Header, _ string) {
	header.HasTags = true
}

func (tagger *collectedSampleHandler) HandleSample(s *bitflow.Sample, _ string) {
	for _, taggerFunc := range tagger.taggerFuncs {
		taggerFunc(s)
	}
}
