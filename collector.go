package collector

import (
	"errors"
	"fmt"

	bitflow "github.com/antongulenko/go-bitflow"
)

type MetricReader func() bitflow.Value

type MetricReaderMap map[string]MetricReader

var MetricsChanged = errors.New("Metrics of this collector have changed")

// Collector forms a tree-structure of objects that are able to provide regularly
// updated metric values. A collector is first initialized, which can optionally return
// a new list of Collectors that will also be considered. The new collectors will also be
// initialized, until the tree exhausted. Individual collectors can fail the initialization,
// which will not influence the non-failed collectors.
// After the Init() sequence, the Metrics() method is queried to retrieve a list of metrics
// that are delivered by every collector. It may return an empty slice in case of collectors
// that are only there to satisfy dependencies of other collectors.
// Then, the Depends() method is used to build up a dependency graph between the collectors.
// Typically, each collector will returns its parent-collector as sole dependency, but it
// can also return an empty slice or multiple dependencies. All collectors returned from any
// Depends() method must already have been initialized in the Init() sequence.
type Collector interface {

	// Init prepares this collector for collecting metrics and instantiates sub-collectors.
	// If there is no error, the sub-collectors will also be initialized, until there are
	// no more sub-nodes. The metrics in the MetricReaderMap are all stored in one flat list,
	// the keys must be globally unique.
	Init() (subCollectors []Collector, err error)

	// Metrics will only be called after Init() returned successfully. It returns the metrics
	// that are provided by this collector.
	Metrics() MetricReaderMap

	// Depends returns a slice of collectors whose Update() this collector depends on.
	// This means that this collector needs data from those other collectors to perform
	// its Update() routine correctly. Therefore, Update() will be called on those other
	// collectors first. The Depends() methods build up an acyclic dependency graph, whose
	// topological order gives the order of Update() calls.
	Depends() []Collector

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
