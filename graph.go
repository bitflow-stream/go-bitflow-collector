package collector

import (
	"fmt"
	"regexp"

	log "github.com/Sirupsen/logrus"
	"github.com/gonum/graph/topo"
)

type collectorGraph struct {
	nodes    map[*collectorNode]bool
	failed   map[*collectorNode]bool
	filtered map[*collectorNode]bool

	collectors map[Collector]*collectorNode
}

func initCollectorGraph(collectors []Collector) (*collectorGraph, error) {
	g := &collectorGraph{
		nodes:      make(map[*collectorNode]bool),
		failed:     make(map[*collectorNode]bool),
		filtered:   make(map[*collectorNode]bool),
		collectors: make(map[Collector]*collectorNode),
	}
	g.initNodes(collectors)
	if len(g.nodes) == 0 {
		return nil, fmt.Errorf("All %v collectors have failed", len(g.failed))
	}
	if err := g.checkMissingDependencies(); err != nil {
		return nil, err
	}
	// Test if topological sort is possible (no cycles)
	if _, err := topo.Sort(g); err != nil {
		return nil, err
	}
	return g, nil
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
	node := newCollectorNode(col, graph)
	graph.collectors[col] = node

	graph.nodes[node] = true
	children, err := node.init()
	if err == nil {
		graph.initNodes(children)
	} else {
		graph.collectorFailed(node, err)
	}
}

func (graph *collectorGraph) deleteCollector(node *collectorNode) {
	delete(graph.nodes, node)
	delete(graph.failed, node)
	delete(graph.filtered, node)
}

func (graph *collectorGraph) collectorFailed(node *collectorNode, err error) {
	graph.deleteCollector(node)
	graph.failed[node] = true
	log.Warnf("Collector %v failed: %v", node, err)
}

func (graph *collectorGraph) collectorFiltered(node *collectorNode) {
	graph.deleteCollector(node)
	graph.filtered[node] = true
	log.Debugln("Collector", node, "has been filtered")
}

func (graph *collectorGraph) checkMissingDependencies() error {
	for node := range graph.nodes {
		for _, depends := range node.collector.Depends() {
			if _, ok := graph.collectors[depends]; !ok {
				// All collectors (including those from Depends() methods) must be returned by a call to Init()
				return fmt.Errorf("Collector %v depends on a missing collector: %v", node, depends)
			}
		}
	}
	return nil
}

func (graph *collectorGraph) applyMetricFilters(exclude []*regexp.Regexp, include []*regexp.Regexp) {
	for node := range graph.nodes {
		node.applyMetricFilters(exclude, include)
	}
}

func (graph *collectorGraph) pruneEmptyNodes() {
	// For every node, collect the set of nodes that depend on that node
	incoming := make(map[*collectorNode]map[*collectorNode]bool)
	for node := range graph.nodes {
		for _, depends := range node.collector.Depends() {
			dependsNode := graph.resolve(depends)
			m, ok := incoming[dependsNode]
			if !ok {
				m = make(map[*collectorNode]bool)
				incoming[dependsNode] = m
			}
			m[node] = true
		}
	}

	// Obtain topological order of graph
	sorted, err := topo.Sort(graph)
	if err != nil {
		// TODO return error instead. Panic should not happen because of test in init()
		panic(err)
	}

	// Walk "leaf" nodes first
	for _, graphNode := range sorted {
		node := graphNode.(*collectorNode)
		if len(node.metrics) == 0 && len(incoming[node]) == 0 {
			// Nothing depends on this node, and it does not have any metrics
			graph.collectorFiltered(node)
			for _, dependencySet := range incoming {
				delete(dependencySet, node)
			}
		}
	}
}

func (graph *collectorGraph) dependingOn(target *collectorNode) []*collectorNode {
	var nodes []*collectorNode
	for node := range graph.nodes {
		for _, depends := range node.collector.Depends() {
			if depends == target.collector {
				nodes = append(nodes, node)
			}
		}
	}
	return nodes
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

func (graph *collectorGraph) resolve(col Collector) *collectorNode {
	node, ok := graph.collectors[col]
	if !ok {
		// This should not happen after checkMissingDependencies() returns nil
		panic(fmt.Sprintf("Node for collector %v not found!", col))
	}
	return node
}
