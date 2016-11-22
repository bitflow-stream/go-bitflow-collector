package collector

import (
	"fmt"
	"regexp"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/antongulenko/golib"
)

const max_dependency_checks = 5000

type collectorGraph struct {
	nodes    map[*collectorNode]bool
	failed   map[*collectorNode]bool
	filtered map[*collectorNode]bool

	collectors map[Collector]*collectorNode
}

func initCollectorGraph(collectors []Collector) *collectorGraph {
	graph := &collectorGraph{
		nodes:      make(map[*collectorNode]bool),
		failed:     make(map[*collectorNode]bool),
		filtered:   make(map[*collectorNode]bool),
		collectors: make(map[Collector]*collectorNode),
	}
	graph.initNodes(collectors)
	graph.checkDependencies()
	return graph
}

func (graph *collectorGraph) initNodes(collectors []Collector) {
	for _, col := range collectors {
		graph.initNode(col)
	}
}

func (graph *collectorGraph) initNode(col Collector) {
	if _, ok := graph.collectors[col]; ok {
		panic(fmt.Sprintf("Collector %v has already been added to graph", col))
	}
	node := &collectorNode{
		collector: col,
		graph:     graph,
	}
	graph.collectors[col] = node

	graph.nodes[node] = true
	children, err := node.init()
	if err == nil {
		graph.initNodes(children)
	} else {
		log.Warnf("Collector %v failed: %v", col.String(), err)
		graph.collectorFailed(node)
	}
}

func (graph *collectorGraph) deleteCollector(node *collectorNode) {
	delete(graph.nodes, node)
	delete(graph.failed, node)
	delete(graph.filtered, node)
}

func (graph *collectorGraph) collectorFailed(node *collectorNode) {
	graph.deleteCollector(node)
	graph.failed[node] = true
}

func (graph *collectorGraph) collectorFiltered(node *collectorNode) {
	graph.deleteCollector(node)
	graph.filtered[node] = true
}

func (graph *collectorGraph) checkDependencies() {
	timeout := false
	for i := 0; i < max_dependency_checks; i++ {
		if changes := graph.checkMissingDependencies(); changes <= 0 {
			timeout = true
			break
		}
	}
	if timeout {
		log.Fatalln("Dependencies still changing after checking", max_dependency_checks, "times")
	}
}

func (graph *collectorGraph) checkMissingDependencies() (changes int) {
	for node := range graph.nodes {
		for _, depends := range node.collector.Depends() {
			node, ok := graph.collectors[depends]
			if ok {
				_, ok = graph.nodes[node]
				if !ok {
					log.Debugln("Collector ", node, "has failed/filtered dependency:", depends)
				}
			} else {
				log.Errorln("Collector ", node, "has unresolved dependency:", depends)
			}
			if !ok {
				graph.collectorFiltered(node)
				changes++
				break
			}
		}
	}
	return
}

func (graph *collectorGraph) applyMetricFilters(exclude []*regexp.Regexp, include []*regexp.Regexp) {
	for node := range graph.nodes {
		node.applyMetricFilters(exclude, include)
	}
}

func (graph *collectorGraph) pruneEmptyNodes() {
	for node := range graph.nodes {
		if len(node.metrics) == 0 {
			graph.collectorFiltered(node)
		}
	}
	for node := range graph.nodes {
		filtered := false
		// TODO find out if any other node depends on this one
		if filtered {
			graph.collectorFiltered(node)
		}
	}
}

func (graph *collectorGraph) listMetricNames() []string {
	metrics := make(map[string]bool)
	graph.fillMetricNames(metrics)
	res := make([]string, 0, len(metrics))
	for metric := range metrics {
		res = append(res, metric)
	}
	return res
}

func (graph *collectorGraph) fillMetricNames(all map[string]bool) {
	for node := range graph.nodes {
		for metric := range node.metrics {
			if _, ok := all[metric]; ok {
				log.Errorln("Metric", metric, "is delivered by multiple collectors!")
			}
			all[metric] = true
		}
	}
}

func (graph *collectorGraph) getMetrics() (res MetricSlice) {
	for node := range graph.nodes {
		for name, reader := range node.metrics {
			res = append(res, &Metric{
				name:   name,
				reader: reader,
			})
		}
	}
	return
}

func (graph *collectorGraph) startParallelUpdates(wg *sync.WaitGroup, stopper *golib.Stopper, collectInterval time.Duration) {
	for node := range graph.nodes {
		wg.Add(1)
		go graph.updateCollector(wg, node, stopper, collectInterval)
	}
	for node := range graph.failed {
		wg.Add(1)
		go graph.updateCollector(wg, node, stopper, FilteredCollectorCheckInterval)
	}
	wg.Add(1)
	go graph.watchFailedCollectors(wg, stopper)
}

func (source *collectorGraph) updateCollector(wg *sync.WaitGroup, node *collectorNode, stopper *golib.Stopper, interval time.Duration) {
	defer wg.Done()
	for {
		err := node.collector.Update()
		if err == MetricsChanged {
			log.Warnln("Metrics of", node, "have changed! Restarting metric collection.")
			stopper.Stop()
			return
		} else if err != nil {
			log.Warnln("Update of", node, "failed:", err)
		}
		if stopper.Stopped(interval) {
			return
		}
	}
}

func (graph *collectorGraph) watchFailedCollectors(wg *sync.WaitGroup, stopper *golib.Stopper) {
	defer wg.Done()
	for {
		for node := range graph.failed {
			if _, err := node.init(); err == nil {
				log.Warnln("Collector", node, "is not failing anymore. Restarting metric collection.")
				stopper.Stop()
				return
			}
			if stopper.IsStopped() {
				return
			}
		}
		if stopper.Stopped(FailedCollectorCheckInterval) {
			return
		}
	}
}
