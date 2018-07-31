package collector

import (
	"fmt"
	"regexp"
	"time"

	"sync"

	"github.com/gonum/graph"
	"github.com/gonum/graph/topo"
	log "github.com/sirupsen/logrus"
)

type collectorGraph struct {
	nodes      map[*collectorNode]bool
	failed     map[*collectorNode]bool
	failedList []*collectorNode
	filtered   map[*collectorNode]bool

	collectors       map[Collector]*collectorNode
	modificationLock sync.Mutex
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
		// This collector has already been added
		return
	}
	node := newCollectorNode(col, graph)
	graph.collectors[col] = node

	graph.nodes[node] = true
	children, err := node.init()
	if err == nil {
		graph.initNodes(children)
	} else {
		graph.collectorFailed(node)
		log.Warnf("Collector %v failed: %v", node, err)
	}
}

func (graph *collectorGraph) deleteCollector(node *collectorNode) {
	delete(graph.nodes, node)
	delete(graph.filtered, node)
	delete(graph.failed, node)
	// Keep the collector in the graph.collectors map in case it needs to be
	// accessed through resolve()
}

func (graph *collectorGraph) collectorFailed(node *collectorNode) {
	delete(graph.nodes, node)
	delete(graph.filtered, node)
	if !graph.failed[node] {
		graph.failed[node] = true
		graph.failedList = append(graph.failedList, node)
	}
}

func (graph *collectorGraph) collectorFiltered(node *collectorNode) {
	if !graph.failed[node] {
		delete(graph.nodes, node)
		graph.filtered[node] = true
	}
}

func (graph *collectorGraph) collectorUpdateFailed(node *collectorNode) {
	// This means the collector Init() method was successful, but then Update() returned errors too many times.
	graph.modificationLock.Lock()
	defer graph.modificationLock.Unlock()
	graph.collectorFailed(node)
	graph.pruneAndRepair()
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

func (graph *collectorGraph) applyUpdateFrequencies(frequencies map[*regexp.Regexp]time.Duration) {
	for regex, freq := range frequencies {
		count := 0
		for node := range graph.nodes {
			if regex.MatchString(node.String()) {
				node.UpdateFrequency = freq
				count++
			}
		}
		log.Debugf("Update frequency %v applied to %v nodes matching %v", freq, count, regex.String())
	}
}

func (graph *collectorGraph) dependsOnFailedOrFiltered(node *collectorNode) bool {
	for _, dependencyCol := range node.collector.Depends() {
		dependency := graph.resolve(dependencyCol)
		if !graph.nodes[dependency] {
			return true
		}
	}
	return false
}

func (graph *collectorGraph) pruneAndRepair() {
	// Obtain topological order of graph
	sorted := sortGraph(graph)

	// Walk "root" nodes first: delete nodes with failed dependencies
	// Since we walk the sorted graph, all transitive dependencies will be deleted as well
	for i, node := range sorted {
		if graph.dependsOnFailedOrFiltered(node) {
			log.Warnln("Deleting collector", node, "because of a failed/filtered dependency")
			graph.deleteCollector(node)
			sorted[i] = nil
		}
	}

	// Walk "leaf" nodes first
	incoming := graph.reverseDependencies()
	for i := len(sorted) - 1; i >= 0; i-- {
		if sorted[i] == nil {
			continue
		}
		node := sorted[i]
		if len(node.metrics) == 0 && len(incoming[node]) == 0 {
			// Nothing depends on this node, and it does not have any metrics
			graph.collectorFiltered(node)
			for _, dependencySet := range incoming {
				delete(dependencySet, node)
			}
		}
	}
}

// For every node, collect the set of nodes that depend on that node
func (graph *collectorGraph) reverseDependencies() map[*collectorNode]map[*collectorNode]bool {
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
	return incoming
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

func sortGraph(graph graph.Directed) []*collectorNode {
	sortedGraph, err := topo.Sort(graph)
	if err != nil {
		// Should not happen, graph should already be asserted acyclic
		panic(err)
	}
	sorted := make([]*collectorNode, len(sortedGraph))
	for j, node := range sortedGraph {
		sorted[len(sortedGraph)-1-j] = node.(*collectorNode)
	}
	return sorted
}

func createCollectorSubGraph(nodes []graph.Node) *collectorGraph {
	newGraph := &collectorGraph{
		nodes:      make(map[*collectorNode]bool),
		failed:     make(map[*collectorNode]bool),
		filtered:   make(map[*collectorNode]bool),
		collectors: make(map[Collector]*collectorNode),
	}
	for _, graphNode := range nodes {
		node := graphNode.(*collectorNode)
		newGraph.nodes[node] = true
		newGraph.collectors[node.collector] = node
	}
	return newGraph
}

func (g *collectorGraph) sortedFilteredNodes() []*collectorNode {
	if len(g.filtered) == 0 {
		return nil
	} else if len(g.filtered) == 1 {
		var res []*collectorNode
		for node := range g.filtered {
			res = append(res, node)
		}
		return res
	}

	// Sort the graph including filtered, failed and unfiltered nodes,
	// then extract only the filtered ones in the correct order
	res := make([]*collectorNode, 0, len(g.filtered))
	fullGraph := createCollectorSubGraph(makeNodeList(g.nodes, g.filtered, g.failed))
	sorted := sortGraph(fullGraph)
	for _, node := range sorted {
		if g.filtered[node] {
			res = append(res, node)
		}
	}
	return res
}

func makeNodeList(sets ...map[*collectorNode]bool) []graph.Node {
	var res []graph.Node
	for _, set := range sets {
		for node := range set {
			res = append(res, node)
		}
	}
	return res
}

func (g *collectorGraph) getRootsAndLeafs() (roots []*collectorNode, leafs []*collectorNode) {
	reverse := g.reverseDependencies()
	for node := range g.nodes {
		if len(node.collector.Depends()) == 0 {
			roots = append(roots, node)
		}
		if len(reverse[node]) == 0 {
			leafs = append(leafs, node)
		}
	}
	return
}
