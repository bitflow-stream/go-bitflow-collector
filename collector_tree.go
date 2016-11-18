package collector

import (
	"regexp"
	"sync"
	"time"

	"github.com/antongulenko/golib"
)

type collectorNode struct {
	collector Collector

	// Set in init()
	children       []*collectorNode
	failedChildren []*collectorNode
	allMetrics     MetricReaderMap

	// Set in pruneEmptyNodes()
	filteredChildren []*collectorNode

	// set in initMetrics()
	metrics []*Metric
}

func (node *collectorNode) init() error {
	if err := node.initCollector(); err != nil {
		return err
	}
	node.initChildren()
}

func (node *collectorNode) initCollector() error {
	node.children = node.children[0:0]
	node.failedChildren = node.failedChildren[0:0]

	metrics, children, err := node.collector.Init()
	if err != nil {
		return err
	}
	err = node.collector.Update()
	if err != nil {
		return err
	}
	node.allMetrics = metrics
}

func (node *collectorNode) initChildren() {
	for _, child := range children {
		childNode := &collectorNode{
			collector: child,
		}
		if err := childNode.init(); err != nil {
			node.failedChildren = append(node.failedChildren, childNode)
			log.Warnf("Collector %v failed: %v", child.String(), err)
		} else {
			node.children = append(node.children, childNode)
		}
	}
}

func (node *collectorNode) applyMetricFilters(exclude []*regexp.Regexp, include []*regexp.Regexp) {
	filtered := node.getFilteredMetrics(exclude, include)
	for _, name := range node.allMetrics {
		if !filtered[name] {
			delete(node.allMetrics, name)
		}
	}
	for _, child := range node.children {
		child.applyMetricFilters(exclude, include)
	}
}

func (node *collectorNode) pruneEmptyNodes() {
	for i, child := range node.children {
		child.pruneEmptyNodes()
		if len(child.children) == 0 && len(child.allMetrics) == 0 {
			node.children[i] = nil
			log.Println("Removing filtered collector:", child.collector.String())
		}
	}
}

func (node *collectorNode) getFilteredMetrics(exclude []*regexp.Regexp, include []*regexp.Regexp) map[string]bool {
	filtered = make(map[string]bool, 0, len(node.metrics))
	for metric := range node.allMetrics {
		excluded := false
		for _, regex := range exclude {
			if excluded = regex.MatchString(metric); excluded {
				break
			}
		}
		if !excluded && len(include) > 0 {
			excluded = true
			for _, regex := range include {
				if excluded = !regex.MatchString(metric); !excluded {
					break
				}
			}
		}
		if !excluded {
			filtered[metrics] = true
		}
	}
	return filterd
}

func (node *collectorNode) listMetricNames() []string {
	metrics := make(map[string]bool)
	node.fillMetricNames(metrics)
	res := make([]string, 0, len(metrics))
	for metric := range metrics {
		res = append(res, metric)
	}
	return res
}

func (node *collectorNode) fillMetricNames(all map[string]bool) {
	for metric := range node.allMetrics {
		if _, ok := all[metric]; ok {
			log.Errorln("Metric", metric, "is delivered by multiple collectors!")
		}
		all[metric] = true
	}
	for _, child := range node.children {
		child.fillMetricNames(all)
	}
}

func (node *collectorNode) initMetrics() {
	node.metrics = node.metrics[0:0]
	for name, reader := range node.allMetrics {
		metric := &Metric{
			name:   name,
			reader: reader,
		}
		node.metrics = append(node.metrics, metric)
	}
	for _, child := range node.children {
		child.initMetrics()
	}
}

func (node *collectorNode) listMetrics() []*Metric {
	metrics := node.metrics
	for _, child := range node.children {
		metrics = append(metrics, child.listMetrics()...)
	}
	return metrics
}

func (node *collectorNode) startParallelUpdates(wg *sync.WaitGroup, stopper *golib.Stopper) {
	for _, collector := range source.children {
		var interval time.Duration
		if _, ok := collectors[collector]; ok {
			interval = source.CollectInterval
		} else {
			interval = FilteredCollectorCheckInterval
		}
		wg.Add(1)
		go source.updateCollector(wg, collector, stopper, interval)
	}
	for _, failed := range source.failedCollectors {
		wg.Add(1)
		go source.watchFailedCollector(wg, failed, stopper)
	}
}

func (source *CollectorSource) updateCollector(wg *sync.WaitGroup, collector Collector, stopper *golib.Stopper, interval time.Duration) {
	defer wg.Done()
	for {
		err := collector.Update()
		if err == MetricsChanged {
			log.Warnf("Metrics of %v have changed! Restarting metric collection.", collector)
			stopper.Stop()
			return
		} else if err != nil {
			log.Warnln("Update of", collector, "failed:", err)
		}
		if stopper.Stopped(interval) {
			return
		}
	}
}

func (source *CollectorSource) watchFailedCollector(wg *sync.WaitGroup, collector Collector, stopper *golib.Stopper) {
	defer wg.Done()
	for {
		if stopper.Stopped(FailedCollectorCheckInterval) {
			return
		}
		if err := source.initCollector(collector); err == nil {
			log.Warnln("Collector", collector, "is not failing anymore. Restarting metric collection.")
			stopper.Stop()
			return
		}
	}
}

func (node *collectorNode) checkRecoveredChild() string {
	for _, failed := range node.failedChildren {
		if err := failed.initCollector(); err != nil {
			return failed.String()
		}
	}
	for _, child := range node.children {
		return child.checkRecoveredChild()
	}
	return ""
}
