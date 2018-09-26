package collector

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"strconv"

	log "github.com/sirupsen/logrus"
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/encoding/dot"
	"gonum.org/v1/gonum/graph/iterator"
	"gonum.org/v1/gonum/graph/simple"
)

func (g *collectorGraph) WriteGraphPNG(filename string) error {
	dotData, err := dot.Marshal(g, "Collectors", "", "", false)
	if err != nil {
		return err
	}

	log.Debugln("Writing PNG-representation of collector-g to", filename)
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

func (g *collectorGraph) WriteGraphDOT(filename string) error {
	dotData, err := dot.Marshal(g, "Collectors", "", "", false)
	if err != nil {
		return err
	}

	log.Debugln("Writing dot-representation of collector-g to", filename)
	return ioutil.WriteFile(filename, dotData, 0644)
}

// =========================== Implementation of github.com/gonum/graph.Directed and Undirected interfaces ===========================

var _ graph.Graph = new(collectorGraph)
var _ graph.Directed = new(collectorGraph)
var _ graph.Node = new(collectorNode)

func (g *collectorGraph) Has(id int64) bool {
	_, ok := g.nodeIDs[id]
	return ok
}

func (g *collectorGraph) Nodes() graph.Nodes {
	nodes := make([]graph.Node, 0, len(g.nodes))
	for node := range g.nodes {
		nodes = append(nodes, node)
	}
	return iterator.NewOrderedNodes(nodes)
}

func (g *collectorGraph) From(id int64) graph.Nodes {
	node, ok := g.nodeIDs[id]
	if !ok {
		return nil
	}
	var nodes []graph.Node
	for _, depends := range node.collector.Depends() {
		nodes = append(nodes, g.resolve(depends))
	}
	return iterator.NewOrderedNodes(nodes)
}

func (g *collectorGraph) HasEdgeBetween(x, y int64) bool {
	return g.HasEdgeFromTo(x, y) || g.HasEdgeFromTo(y, x)
}

func (g *collectorGraph) Edge(u, v int64) graph.Edge {
	return simple.Edge{
		F: g.nodeIDs[u],
		T: g.nodeIDs[v],
	}
}

func (g *collectorGraph) HasEdgeFromTo(xNode, yNode int64) bool {
	from, ok := g.nodeIDs[xNode]
	if !ok {
		return false
	}
	to, ok := g.nodeIDs[yNode]
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

func (g *collectorGraph) To(id int64) graph.Nodes {
	target, ok := g.nodeIDs[id]
	if !ok {
		return nil
	}

	depending := g.dependingOn(target)
	nodes := make([]graph.Node, len(depending))
	for i, node := range depending {
		nodes[i] = node
	}
	return iterator.NewOrderedNodes(nodes)
}

func (node *collectorNode) ID() int64 {
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
