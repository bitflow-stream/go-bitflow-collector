package collector

import (
	"regexp"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/antongulenko/golib"
)

var __nodeID = 0

type collectorNode struct {
	collector Collector
	graph     *collectorGraph
	uniqueID  int

	metrics MetricReaderMap

	preconditions  []*BoolCondition
	postconditions []*BoolCondition

	UpdateFrequency time.Duration
}

func newCollectorNode(collector Collector, graph *collectorGraph) *collectorNode {
	__nodeID++
	return &collectorNode{
		collector: collector,
		graph:     graph,
		uniqueID:  __nodeID,
	}
}

func (node *collectorNode) String() string {
	return node.collector.String()
}

func (node *collectorNode) init() ([]Collector, error) {
	children, err := node.collector.Init()
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

func (node *collectorNode) loopUpdate(wg *sync.WaitGroup, stopper *golib.Stopper) {
	for _, dependsCol := range node.collector.Depends() {
		depends := node.graph.resolve(dependsCol)
		cond := NewBoolCondition()
		node.preconditions = append(node.preconditions, cond)
		depends.postconditions = append(depends.postconditions, cond)
	}
	freq := node.UpdateFrequency
	frequencyLimited := freq > 0
	var lastUpdate time.Time

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := uint(0); ; i++ {
			for _, cond := range node.preconditions {
				cond.WaitAndUnset()
			}
			if stopper.IsStopped() {
				return
			}

			if frequencyLimited {
				now := time.Now()
				if now.Sub(lastUpdate) >= freq {
					node.update(stopper)
					lastUpdate = now
				}
			} else {
				node.update(stopper)
			}

			for _, cond := range node.postconditions {
				cond.Broadcast()
			}
			if stopper.IsStopped() {
				return
			}
		}
	}()
}

func (node *collectorNode) update(stopper *golib.Stopper) {
	err := node.collector.Update()
	if err == MetricsChanged {
		log.Warnln("Metrics of", node, "have changed! Restarting metric collection.")
		stopper.Stop()
	} else if err != nil {
		// TODO move this collector (and all that depend on it) to the failed collectors
		// see also CollectorSource.watchFilteredCollectors()
		log.Warnln("Update of", node, "failed:", err)
	}
}
