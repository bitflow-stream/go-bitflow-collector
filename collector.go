package collector

import (
	"fmt"

	bitflow "github.com/antongulenko/go-bitflow"
)

var (
	// Can be used to modify collected headers and samples
	CollectedSampleHandler bitflow.ReadSampleHandler

	// Will be passed to CollectedSampleHandler, if set
	CollectorSampleSource = "collected"
)

type MetricReader func() bitflow.Value

type MetricReaderMap map[string]MetricReader

// Collector forms a tree-structure of interfaces that are able to collect metrics.
// Every node has metrics and sub-collectors (both optional, but one of them should be present).
// The tree is built up dynamically in the Init() method.
type Collector interface {

	// Init prepares this collector for collecting metrics and instantiates sub-collectors.
	// If there is no error, the sub-collectors will also be initialized, until there are
	// no more sub-nodes. The metrics in the MetricReaderMap are all stored in one flat list,
	// the keys must be globally unique.
	Init() (metrics MetricReaderMap, subCollectors []Collector, err error)

	// All collectors are updated in the order they were initialized: from the root node, down the tree.
	// An error stops descending down the tree. After a collector has been updated,
	// the metrics associated with that collector will be read. Collectors with only excluded metrics
	// will not be updated.
	Update() error

	// String returns a short but unique label for the collector.
	String() string
}

// ================================= Abstract Collector =================================
type AbstractCollector struct {
	parent *AbstractCollector
	name   string
}

func (source *AbstractCollector) String() string {
	parentName := ""
	if source.parent != nil {
		parentName = source.parent.String() + "/"
	}
	return parentName + source.name
}

// ================================= Collector Slice =================================
type CollectorSlice []Collector

func (s CollectorSlice) Init() (metrics MetricReaderMap, subCollectors []Collector, err error) {
	return nil, s, nil
}

func (s CollectorSlice) Update() error {
	return nil
}

func (s CollectorSlice) String() string {
	return fmt.Sprintf(len(s), "collectors")
}

// ==================== Metric ====================
type Metric struct {
	name   string
	index  int
	sample []bitflow.Value
	reader MetricReader
}

func (metric *Metric) Update() {
	metric.sample[metric.index] = metric.reader()
}
