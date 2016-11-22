package collector

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"regexp"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/antongulenko/golib"
	"github.com/gonum/graph"
	"github.com/gonum/graph/encoding/dot"
)

const max_dependency_checks = 5000

type collectorGraph struct {
	nodes    map[*collectorNode]bool
	failed   map[*collectorNode]bool
	filtered map[*collectorNode]bool

	collectors map[Collector]*collectorNode
}

func initCollectorGraph(collectors []Collector) (*collectorGraph, error) {
	graph := &collectorGraph{
		nodes:      make(map[*collectorNode]bool),
		failed:     make(map[*collectorNode]bool),
		filtered:   make(map[*collectorNode]bool),
		collectors: make(map[Collector]*collectorNode),
	}
	graph.initNodes(collectors)
	if len(graph.nodes) == 0 {
		return nil, fmt.Errorf("All %v collectors have failed", len(graph.failed))
	}
	return graph, graph.checkDependencies()
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

func (graph *collectorGraph) checkDependencies() error {
	timeout := true
	for i := 0; i < max_dependency_checks; i++ {
		if changes := graph.checkMissingDependencies(); changes <= 0 {
			timeout = false
			break
		}
	}
	if timeout {
		return fmt.Errorf("Dependencies still changing after checking %v times", max_dependency_checks)
	}
	return nil
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

func (graph *collectorGraph) WriteGraph(filename string) error {
	dotData, err := dot.Marshal(graph, "Collectors", "", "", false)
	if err != nil {
		return err
	}
	dotFile, err := ioutil.TempFile("", "collector-graph.dot")
	if err != nil {
		return err
	}
	log.Debugln("Writing dot-representation of collector-graph to", dotFile.Name())
	if err := ioutil.WriteFile(dotFile.Name(), dotData, 0644); err != nil {
		return err
	}
	log.Debugln("Writing PNG-representation of collector-graph to", filename)
	_, err = exec.Command("dot", dotFile.Name(), "-Tpng", "-o", filename).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			err = fmt.Errorf("dot output: %v", string(exitErr.Stderr))
		}
	}
	return err
}

// =========================== Implementation of github.com/gonum/graph.Directed interface ===========================

func (graph *collectorGraph) resolve(col Collector) *collectorNode {
	node, ok := graph.collectors[col]
	if !ok {
		panic(fmt.Sprintf("Node for collector %v not found!", col))
	}
	return node
}

func (graph *collectorGraph) Has(graphNode graph.Node) bool {
	if node, ok := graphNode.(*collectorNode); ok {
		_, ok = graph.nodes[node]
		return ok
	}
	return false
}

func (g *collectorGraph) Nodes() []graph.Node {
	nodes := make([]graph.Node, 0, len(g.nodes))
	for node := range g.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

func (g *collectorGraph) From(graphNode graph.Node) []graph.Node {
	node, ok := graphNode.(*collectorNode)
	if !ok {
		return nil
	}

	var nodes []graph.Node
	for _, depends := range node.collector.Depends() {
		nodes = append(nodes, g.resolve(depends))
	}
	return nodes
}

func (g *collectorGraph) HasEdgeBetween(x, y graph.Node) bool {
	return g.HasEdgeFromTo(x, y) || g.HasEdgeFromTo(y, x)
}

func (g *collectorGraph) Edge(u, v graph.Node) graph.Edge {
	node1, ok := u.(*collectorNode)
	if !ok {
		return nil
	}
	node2, ok := v.(*collectorNode)
	if !ok {
		return nil
	}
	return &collectorEdge{
		from: node1,
		to:   node2,
	}
}

func (g *collectorGraph) HasEdgeFromTo(xNode, yNode graph.Node) bool {
	from, ok := xNode.(*collectorNode)
	if !ok {
		return false
	}
	to, ok := yNode.(*collectorNode)
	if !ok {
		return false
	}
	for _, depends := range from.collector.Depends() {
		node := g.resolve(depends)
		if node == to {
			return true
		}
	}
	return false
}

func (g *collectorGraph) To(graphNode graph.Node) []graph.Node {
	target, ok := graphNode.(*collectorNode)
	if !ok {
		return nil
	}

	var nodes []graph.Node
	for node := range g.nodes {
		for _, depends := range node.collector.Depends() {
			if depends == target.collector {
				nodes = append(nodes, node)
			}
		}
	}
	return nodes
}

type collectorEdge struct {
	from *collectorNode
	to   *collectorNode
}

func (edge *collectorEdge) From() graph.Node {
	return edge.from
}

func (edge *collectorEdge) To() graph.Node {
	return edge.to
}

func (edge *collectorEdge) Weight() float64 {
	return 1
}

func (node *collectorNode) ID() int {
	return node.uniqueID
}

func (node *collectorNode) DOTID() string {
	return fmt.Sprintf("\"%v\"", node.collector.String())
}
