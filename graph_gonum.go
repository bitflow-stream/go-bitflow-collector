package collector

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"strconv"

	log "github.com/Sirupsen/logrus"
	"github.com/gonum/graph"
	"github.com/gonum/graph/encoding/dot"
	"github.com/gonum/graph/simple"
)

func (graph *collectorGraph) WriteGraphPNG(filename string) error {
	dotData, err := dot.Marshal(graph, "Collectors", "", "", false)
	if err != nil {
		return err
	}

	log.Debugln("Writing PNG-representation of collector-graph to", filename)
	cmd := exec.Command("dot", "-Tpng", "-o", filename)
	pipe, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	_, err = pipe.Write(dotData)
	if err != nil {
		return err
	}
	if err := pipe.Close(); err != nil {
		log.Warnln("Failed to close pipe to dot-subprocess:", err)
	}

	_, err = cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			err = fmt.Errorf("dot output: %v", string(exitErr.Stderr))
		}
	}
	return err
}

func (graph *collectorGraph) WriteGraphDOT(filename string) error {
	dotData, err := dot.Marshal(graph, "Collectors", "", "", false)
	if err != nil {
		return err
	}

	log.Debugln("Writing dot-representation of collector-graph to", filename)
	return ioutil.WriteFile(filename, dotData, 0644)
}

// =========================== Implementation of github.com/gonum/graph.Directed and Undirected interfaces ===========================

func (graph *collectorGraph) Has(graphNode graph.Node) bool {
	node, ok := graphNode.(*collectorNode)
	if ok {
		_, ok = graph.nodes[node]
	}
	return ok
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
	return simple.Edge{
		F: u,
		T: v,
		W: 1,
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

	depending := g.dependingOn(target)
	nodes := make([]graph.Node, len(depending))
	for i, node := range depending {
		nodes[i] = node
	}
	return nodes
}

func (node *collectorNode) ID() int {
	return node.uniqueID
}

func (node *collectorNode) DOTID() string {
	str := "\"" + node.collector.String()
	if len(node.metrics) > 0 {
		if len(node.metrics) == 1 {
			str += "\n1 metric"
		} else {
			str += "\n" + strconv.Itoa(len(node.metrics)) + " metrics"
		}
	}
	return str + "\""
}
