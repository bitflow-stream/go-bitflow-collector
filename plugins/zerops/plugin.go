package main

import (
	"flag"

	"github.com/bitflow-stream/go-bitflow/script/plugin"
	"github.com/bitflow-stream/go-bitflow/script/reg"
	log "github.com/sirupsen/logrus"
)

func init() {
	// Command line flags should not be used when loading plugins, but the kubernetes client library
	// defines some flags that cannot be avoided otherwise.
	flag.CommandLine = flag.NewFlagSet("", flag.ContinueOnError)
}

func main() {
	log.Fatalln("This package is intended to be loaded as a plugin, not executed directly")
}

// The Symbol to be loaded
var Plugin plugin.BitflowPlugin = new(pluginImpl)

type pluginImpl struct {
}

func (*pluginImpl) Name() string {
	return "zerops-collector-plugin"
}

func (p *pluginImpl) Init(registry reg.ProcessorRegistry) error {
	plugin.LogPluginProcessor(p, "zerops-notify")
	RegisterZeropsDataSourceNotifier("zerops-notify", registry)
	return nil
}
