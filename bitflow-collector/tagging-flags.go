package main

import (
	"flag"

	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/golib"
)

var (
	flagTags golib.KeyValueStringSlice
)

func init() {
	flag.Var(&flagTags, "tag", "All collected samples will have the given tags (key=value) attached.")
}

func registerFlagTags() {
	if len(flagTags.Keys) > 0 {
		sampleTagger.taggerFuncs = append(sampleTagger.taggerFuncs, flagTagger)
	}
}

func flagTagger(s *bitflow.Sample) {
	for i, tagName := range flagTags.Keys {
		s.SetTag(tagName, flagTags.Values[i])
	}
}
