package collector

import (
	"errors"
	"sort"
	"sync"

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

	// MetricsChanged should check if the collector can produce a different set of metrics, and if so,
	// the MetricsChanged error instance. Many collectors have a fixed set of metrics, so nil should
	// be returned here (as in AbstractCollector). Collectors that potentially return MetricsChanged from
	// Update(), should use Update() as implementation for MetricsChanged().
	MetricsChanged() error

	// String returns a short but unique label for the colldector.
	String() string
}

// ================================= Abstract Collector =================================
type AbstractCollector struct {
	Parent *AbstractCollector
	Name   string
}

func RootCollector(name string) AbstractCollector {
	return AbstractCollector{
		Name: name,
	}
}

func (col *AbstractCollector) Child(name string) AbstractCollector {
	return AbstractCollector{
		Parent: col,
		Name:   name,
	}
}

func (col *AbstractCollector) String() string {
	parentName := ""
	if col.Parent != nil {
		parentName = col.Parent.String() + "/"
	}
	return parentName + col.Name
}

func (col *AbstractCollector) Init() ([]Collector, error) {
	return nil, nil
}

func (col *AbstractCollector) Depends() []Collector {
	return nil
}

func (col *AbstractCollector) Metrics() MetricReaderMap {
	return nil
}

func (col *AbstractCollector) Update() error {
	return nil
}

func (col *AbstractCollector) MetricsChanged() error {
	return nil
}

// ==================== Metric ====================
type Metric struct {
	name   string
	index  int
	sample []bitflow.Value
	reader MetricReader

	// The use of this RWMutex is inverted: the Metric.Update() routine uses
	// the read-lock, even though it writes data, because we every instance of Metric
	// accesses another index in the []bitflow.Value slice. The copy function returned by
	// ConstructSample() uses the writer-lock, even though it reads (copies) the sample slice,
	// because we need its access to be exclusive from the write accesses by all Metric instances.
	sampleLock *sync.RWMutex
}

func (metric *Metric) Update() {
	metric.sampleLock.RLock()
	defer metric.sampleLock.RUnlock()

	metric.sample[metric.index] = metric.reader()
}

// ==================== Metric Slice ====================
type MetricSlice []*Metric

func (s MetricSlice) Len() int {
	return len(s)
}

func (s MetricSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s MetricSlice) Less(i, j int) bool {
	return s[i].name < s[j].name
}

func (s MetricSlice) ConstructSample(source *CollectorSource) ([]string, func() []bitflow.Value) {
	var sampleLock sync.RWMutex // See comment at Metric.sampleLock

	sort.Sort(s)
	fields := make([]string, len(s))
	values := make([]bitflow.Value, len(s))
	for i, metric := range s {
		fields[i] = metric.name
		metric.index = i
		metric.sample = values
		metric.sampleLock = &sampleLock
	}

	valueLen := len(values)
	valueCap := bitflow.RequiredValues(valueLen, source.OutgoingSink)
	return fields, func() []bitflow.Value {
		sampleCopy := make([]bitflow.Value, valueLen, valueCap)
		sampleLock.Lock()
		defer sampleLock.Unlock()
		copy(sampleCopy, values)
		return sampleCopy
	}
}

func (s MetricSlice) UpdateAll() {
	for _, metric := range s {
		metric.Update()
	}
}
