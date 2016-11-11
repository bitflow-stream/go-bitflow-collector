package collector

import (
	"fmt"

	bitflow "github.com/antongulenko/go-bitflow"
)

// Can be used to modify collected headers and samples
var CollectedSampleHandler bitflow.ReadSampleHandler

const CollectorSampleSource = "collected"

// ==================== Metric ====================
type Metric struct {
	Name   string
	index  int
	sample []bitflow.Value
}

func (metric *Metric) Set(val bitflow.Value) {
	metric.sample[metric.index] = val
}

// ==================== Collector ====================
type Collector interface {
	Init() error
	Collect(metric *Metric) error
	Update() error
	SupportedMetrics() []string
	SupportsMetric(metric string) bool
}

var collectorRegistry = make(map[Collector]bool)

func RegisterCollector(collector Collector) {
	collectorRegistry[collector] = true
}

// ================================= Abstract Collector =================================
type AbstractCollector struct {
	metrics []*CollectedMetric
	Readers map[string]MetricReader // Must be filled in Init() implementations
	Notify  map[string]CollectNotification
	name    string
}

type CollectedMetric struct {
	*Metric
	MetricReader
}

type CollectNotification func()
type MetricReader func() bitflow.Value

func (source *AbstractCollector) Reset(parent interface{}) {
	source.metrics = nil
	source.Readers = nil
	source.Notify = make(map[string]CollectNotification)
	source.name = fmt.Sprintf("%T", parent)
}

func (source *AbstractCollector) SupportedMetrics() (res []string) {
	res = make([]string, 0, len(source.Readers))
	for metric, _ := range source.Readers {
		res = append(res, metric)
	}
	return
}

func (source *AbstractCollector) SupportsMetric(metric string) bool {
	_, ok := source.Readers[metric]
	return ok
}

func (source *AbstractCollector) Collect(metric *Metric) error {
	tags := make([]string, 0, len(source.Readers))
	for metricName, reader := range source.Readers {
		if metric.Name == metricName {
			source.metrics = append(source.metrics, &CollectedMetric{
				Metric:       metric,
				MetricReader: reader,
			})
			if notifier, ok := source.Notify[metric.Name]; ok {
				notifier()
			}
			return nil
		}
		tags = append(tags, metric.Name)
	}
	return fmt.Errorf("Cannot handle metric %v, expected one of %v", metric.Name, tags)
}

func (source *AbstractCollector) UpdateMetrics() {
	for _, metric := range source.metrics {
		metric.Set(metric.MetricReader())
	}
}

func (source *AbstractCollector) String() string {
	l := len(source.metrics)
	if l > 0 {
		return fmt.Sprintf("%s (%v metrics)", source.name, len(source.metrics))
	} else {
		return source.name
	}
}
