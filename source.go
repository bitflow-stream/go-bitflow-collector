package collector

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/golib"
)

var (
	// Can be used to modify collected headers and samples
	CollectedSampleHandler bitflow.ReadSampleHandler

	// Will be passed to CollectedSampleHandler, if set
	CollectorSampleSource = "collected"
)

const (
	FailedCollectorCheckInterval   = 5 * time.Second
	FilteredCollectorCheckInterval = 3 * time.Second
)

type CollectorSource struct {
	bitflow.AbstractMetricSource

	RootCollectors    []Collector
	CollectInterval   time.Duration
	UpdateFrequencies map[*regexp.Regexp]time.Duration
	SinkInterval      time.Duration
	ExcludeMetrics    []*regexp.Regexp
	IncludeMetrics    []*regexp.Regexp

	loopTask *golib.LoopTask
}

func (source *CollectorSource) String() string {
	return fmt.Sprintf("CollectorSource (%v collectors)", len(source.RootCollectors))
}

func (source *CollectorSource) Start(wg *sync.WaitGroup) golib.StopChan {
	// TODO integrate golib.StopChan/LoopTask and golib.Stopper
	source.loopTask = golib.NewErrLoopTask(source.String(), func(stop golib.StopChan) error {
		var collectWg sync.WaitGroup
		stopper, err := source.collect(&collectWg)
		if err != nil {
			return err
		}
		select {
		case <-stopper.Wait():
		case <-stop:
		}
		stopper.Stop()
		collectWg.Wait()
		return nil
	})
	source.loopTask.StopHook = func() {
		source.CloseSink(wg)
	}
	return source.loopTask.Start(wg)
}

func (source *CollectorSource) Stop() {
	source.loopTask.Stop()
}

func (source *CollectorSource) collect(wg *sync.WaitGroup) (*golib.Stopper, error) {
	graph, err := source.createFilteredGraph()
	if err != nil {
		return nil, err
	}

	metrics := graph.getMetrics()
	fields, values := metrics.ConstructSample()
	log.Println("Collecting", len(metrics), "metrics through", len(graph.collectors), "collectors")
	graph.applyUpdateFrequencies(source.UpdateFrequencies)

	stopper := golib.NewStopper()
	source.startUpdates(wg, stopper, graph)
	source.watchFilteredCollectors(wg, stopper, graph)
	source.watchFailedCollectors(wg, stopper, graph)
	wg.Add(1)
	go source.sinkMetrics(wg, metrics, fields, values, stopper)
	return stopper, nil
}

func (source *CollectorSource) createGraph() (*collectorGraph, error) {
	return initCollectorGraph(source.RootCollectors)
}

func (source *CollectorSource) createFilteredGraph() (*collectorGraph, error) {
	graph, err := source.createGraph()
	if err != nil {
		return nil, err
	}
	graph.applyMetricFilters(source.ExcludeMetrics, source.IncludeMetrics)
	graph.pruneAndRepair()
	return graph, nil
}

func (source *CollectorSource) sinkMetrics(wg *sync.WaitGroup, metrics MetricSlice, fields []string, values []bitflow.Value, stopper *golib.Stopper) {
	defer wg.Done()

	header := &bitflow.Header{Fields: fields}
	if handler := CollectedSampleHandler; handler != nil {
		handler.HandleHeader(header, CollectorSampleSource)
	}
	// TODO source.CheckSink() should be called in Start()
	sink := source.OutgoingSink

	for {
		metrics.UpdateAll()
		sample := &bitflow.Sample{
			Time:   time.Now(),
			Values: values,
		}
		if handler := CollectedSampleHandler; handler != nil {
			handler.HandleSample(sample, CollectorSampleSource)
		}
		if err := sink.Sample(sample, header); err != nil {
			log.Warnln("Failed to sink", len(values), "metrics:", err)
		}
		if stopper.Stopped(source.SinkInterval) {
			return
		}
	}
}

func (source *CollectorSource) startUpdates(wg *sync.WaitGroup, stopper *golib.Stopper, graph *collectorGraph) {
	roots, leafs := graph.getRootsAndLeafs()
	log.Debugln("Root collectors:", len(roots), roots)
	log.Debugln("Leaf collectors:", len(leafs), leafs)

	rootConditions := make([]*BoolCondition, len(roots))
	leafConditions := make([]*BoolCondition, len(leafs))
	for i, root := range roots {
		cond := NewBoolCondition()
		rootConditions[i] = cond
		root.preconditions = append(root.preconditions, cond)
	}
	for i, leaf := range leafs {
		cond := NewBoolCondition()
		leafConditions[i] = cond
		leaf.postconditions = append(leaf.postconditions, cond)
	}

	// Prepare all nodes for updates
	for node := range graph.nodes {
		node.loopUpdate(wg, stopper)
	}

	// Wait for first update of all collectors
	log.Debugln("Performing initial collector updates...")
	source.setAll(rootConditions)
	for _, cond := range leafConditions {
		cond.Wait()
	}
	log.Debugln("Initial updates complete, now starting background updates")

	// Now do regular updated in the background
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			// Make sure to wake up and stop all update routines
			stopper.Stop()
			for node := range graph.nodes {
				source.setAll(node.preconditions)
			}
		}()
		for {
			source.setAll(rootConditions)
			if stopper.Stopped(source.CollectInterval) {
				break
			}
		}
	}()
}

func (source *CollectorSource) setAll(conds []*BoolCondition) {
	for _, cond := range conds {
		cond.Broadcast()
	}
}

func (source *CollectorSource) watchFilteredCollectors(wg *sync.WaitGroup, stopper *golib.Stopper, graph *collectorGraph) {
	filtered := graph.sortedFilteredNodes()
	if len(filtered) == 0 {
		return
	}
	log.Debugln("Watching filtered collectors:", filtered)

	source.loopCheck(wg, stopper, filtered, FilteredCollectorCheckInterval, func(node *collectorNode) {
		err := node.collector.MetricsChanged()
		if err == MetricsChanged {
			log.Warnln("Metrics of", node, " (filtered) have changed! Restarting metric collection.")
			stopper.Stop()
		} else if err != nil {
			// TODO move this collector (and all that depend on it) to the failed collectors
			// see also collectorNode.update()
			log.Warnln("Update of ", node, " (filtered) failed:", err)
		}
	})
}

func (source *CollectorSource) watchFailedCollectors(wg *sync.WaitGroup, stopper *golib.Stopper, graph *collectorGraph) {
	if len(graph.failed) == 0 {
		return
	}
	failed := make([]*collectorNode, 0, len(graph.failed))
	for node := range graph.failed {
		failed = append(failed, node)
	}
	log.Debugln("Watching failed collectors:", failed)

	source.loopCheck(wg, stopper, failed, FailedCollectorCheckInterval, func(node *collectorNode) {
		if _, err := node.init(); err == nil {
			log.Warnln("Collector", node, "is not failing anymore. Restarting metric collection.")
			stopper.Stop()
		}
	})
}

func (source *CollectorSource) loopCheck(wg *sync.WaitGroup, stopper *golib.Stopper, nodes []*collectorNode, interval time.Duration, check func(*collectorNode)) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			for _, node := range nodes {
				check(node)
				if stopper.IsStopped() {
					return
				}
			}
			if stopper.Stopped(interval) {
				return
			}
		}
	}()
}

func (source *CollectorSource) PrintMetrics() error {
	graph, err := initCollectorGraph(source.RootCollectors)
	if err != nil {
		return err
	}
	all := graph.listMetricNames()
	graph.applyMetricFilters(source.ExcludeMetrics, source.IncludeMetrics)
	filtered := graph.listMetricNames()
	sort.Strings(all)
	sort.Strings(filtered)
	i := 0
	for _, metric := range all {
		isIncluded := i < len(filtered) && filtered[i] == metric
		if isIncluded {
			i++
		}
		if !isIncluded {
			fmt.Println(metric, "(excluded)")
		} else {
			fmt.Println(metric)
		}
	}
	return nil
}

func (source *CollectorSource) getGraphForPrinting(fullGraph bool) (*collectorGraph, error) {
	if fullGraph {
		return source.createGraph()
	} else {
		return source.createFilteredGraph()
	}
}

func (source *CollectorSource) PrintGraph(file string, fullGraph bool) error {
	graph, err := source.getGraphForPrinting(fullGraph)
	if err != nil {
		return err
	}
	if !strings.HasSuffix(file, ".png") {
		file += ".png"
	}
	if err := graph.WriteGraphPNG(file); err != nil {
		return fmt.Errorf("Failed to create graph image: %v", err)
	}
	return nil
}

func (source *CollectorSource) PrintGraphDot(file string, fullGraph bool) error {
	graph, err := source.getGraphForPrinting(fullGraph)
	if err != nil {
		return err
	}
	if !strings.HasSuffix(file, ".dot") {
		file += ".dot"
	}
	if err := graph.WriteGraphDOT(file); err != nil {
		return fmt.Errorf("Failed to create graph dot file: %v", err)
	}
	return nil
}
