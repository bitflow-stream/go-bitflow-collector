package collector

import "regexp"

var __nodeID = 0

type collectorNode struct {
	collector Collector
	graph     *collectorGraph
	uniqueID  int

	metrics MetricReaderMap
}

func (node *collectorNode) String() string {
	return node.collector.String()
}

func newCollectorNode(collector Collector, graph *collectorGraph) *collectorNode {
	__nodeID++
	return &collectorNode{
		collector: collector,
		graph:     graph,
		uniqueID:  __nodeID,
	}
}

func (node *collectorNode) init() ([]Collector, error) {
	children, err := node.collector.Init()
	if err != nil {
		return nil, err
	}
	err = node.collector.Update()
	if err != nil {
		return nil, err
	}
	node.metrics = node.collector.Metrics()
	return children, nil
}

func (node *collectorNode) applyMetricFilters(exclude []*regexp.Regexp, include []*regexp.Regexp) {
	filtered := node.getFilteredMetrics(exclude, include)
	for name := range node.metrics {
		if !filtered[name] {
			delete(node.metrics, name)
		}
	}
}

func (node *collectorNode) getFilteredMetrics(exclude []*regexp.Regexp, include []*regexp.Regexp) map[string]bool {
	filtered := make(map[string]bool)
	for metric := range node.metrics {
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
			filtered[metric] = true
		}
	}
	return filtered
}
