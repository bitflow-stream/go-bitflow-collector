package collector

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/antongulenko/golib"
	"github.com/bitflow-stream/go-bitflow"
	log "github.com/sirupsen/logrus"
)

// For the update and sink interval, the sleep duration accuracy is increased by
// waking up regularly and checking of the desired sleep time has passed already.
// This stabilizes sleep times in high-CPU and low-priority situations.
const timeoutLoopFactor = 0.1

type SampleSource struct {
	bitflow.AbstractSampleSource

	RootCollectors     []Collector
	CollectInterval    time.Duration
	UpdateFrequencies  map[*regexp.Regexp]time.Duration
	SinkInterval       time.Duration
	ExcludeMetrics     []*regexp.Regexp
	IncludeMetrics     []*regexp.Regexp
	DisabledCollectors []string

	FailedCollectorCheckInterval   time.Duration
	FilteredCollectorCheckInterval time.Duration

	loopTask       *golib.LoopTask
	currentMetrics []string
}

func (source *SampleSource) String() string {
	return fmt.Sprintf("CollectorSource (%v root-collectors)", len(source.RootCollectors))
}

func (source *SampleSource) CurrentMetrics() []string {
	return source.currentMetrics
}

func (source *SampleSource) Start(wg *sync.WaitGroup) golib.StopChan {
	for name, val := range map[string]time.Duration{
		"CollectInterval":                source.CollectInterval,
		"SinkInterval":                   source.SinkInterval,
		"FailedCollectorCheckInterval":   source.FailedCollectorCheckInterval,
		"FilteredCollectorCheckInterval": source.FilteredCollectorCheckInterval,
	} {
		if val <= 0 {
			return golib.NewStoppedChan(fmt.Errorf("The field CollectorSource.%v must be set to a positive value (have %v)", name, val))
		}
	}

	source.loopTask = &golib.LoopTask{
		Description: source.String(),
		StopHook: func() {
			source.CloseSinkParallel(wg)
		},
		Loop: func(loopStop golib.StopChan) error {
			var collectWg sync.WaitGroup
			collectionStop, err := source.collect(&collectWg)
			if err != nil {
				return err
			}
			select {
			case <-collectionStop.WaitChan():
			case <-loopStop.WaitChan():
			}
			collectionStop.Stop()
			collectWg.Wait()
			return nil
		},
	}
	return source.loopTask.Start(wg)
}

func (source *SampleSource) Close() {
	source.loopTask.Stop()
}

func (source *SampleSource) collect(wg *sync.WaitGroup) (golib.StopChan, error) {
	graph, err := source.createFilteredGraph()
	if err != nil {
		return golib.StopChan{}, err
	}

	metrics := graph.getMetrics()
	fields, getValues := metrics.ConstructSample(source)
	log.Println("Collecting", len(metrics), "metrics through", len(graph.collectors), "collectors")
	graph.applyUpdateFrequencies(source.UpdateFrequencies)

	stopper := golib.NewStopChan()
	source.startUpdates(wg, stopper, graph)
	source.watchFilteredCollectors(wg, stopper, graph)
	source.watchFailedCollectors(wg, stopper, graph)
	wg.Add(1)
	go source.sinkMetrics(wg, metrics, fields, getValues, stopper)
	return stopper, nil
}

func (source *SampleSource) createGraph() (*collectorGraph, error) {
	roots := make([]Collector, 0, len(source.RootCollectors))
	for _, root := range source.RootCollectors {
		name := root.String()
		isEnabled := true
		for _, disabled := range source.DisabledCollectors {
			// Disabled root collectors are ignored immediately
			if name == disabled {
				isEnabled = false
				break
			}
		}
		if isEnabled {
			roots = append(roots, root)
		} else {
			log.Debugln("Disabling root collector", name)
		}
	}
	return initCollectorGraph(roots)
}

func (source *SampleSource) createFilteredGraph() (*collectorGraph, error) {
	graph, err := source.createGraph()
	if err != nil {
		return nil, err
	}
	graph.applyMetricFilters(source.ExcludeMetrics, source.IncludeMetrics)
	graph.applyCollectorFilters(source.DisabledCollectors)
	graph.pruneAndRepair()
	return graph, nil
}

func (source *SampleSource) sinkMetrics(wg *sync.WaitGroup, metrics MetricSlice, fields []string, getValues func() []bitflow.Value, stopper golib.StopChan) {
	defer wg.Done()

	source.currentMetrics = fields
	header := &bitflow.Header{Fields: fields}
	sink := source.GetSink()

	sinkTime := time.Now()
	for {
		metrics.UpdateAll()
		values := getValues()
		sample := &bitflow.Sample{
			Time:   time.Now(),
			Values: values,
		}
		if err := sink.Sample(sample, header); err != nil {
			log.Warnln("Failed to sink", len(values), "metrics:", err)
		}
		if !stopper.WaitTimeoutPrecise(source.SinkInterval, timeoutLoopFactor, &sinkTime) {
			return
		}
	}
}

func (source *SampleSource) startUpdates(wg *sync.WaitGroup, stopper golib.StopChan, graph *collectorGraph) {
	roots, leafs := graph.getRootsAndLeafs()
	log.Debugln("Root collectors:", len(roots), roots)
	log.Debugln("Leaf collectors:", len(leafs), leafs)

	rootConditions := make([]*golib.BoolCondition, len(roots))
	leafConditions := make([]*golib.BoolCondition, len(leafs))
	for i, root := range roots {
		cond := golib.NewBoolCondition()
		rootConditions[i] = cond
		root.preconditions = append(root.preconditions, cond)
	}
	for i, leaf := range leafs {
		cond := golib.NewBoolCondition()
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

	// Now do regular updates in the background
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
		triggerTime := time.Now()
		for {
			source.setAll(rootConditions)
			if !stopper.WaitTimeoutPrecise(source.CollectInterval, timeoutLoopFactor, &triggerTime) {
				break
			}
		}
	}()
}

func (source *SampleSource) setAll(conditions []*golib.BoolCondition) {
	for _, cond := range conditions {
		cond.Broadcast()
	}
}

func (source *SampleSource) watchFilteredCollectors(wg *sync.WaitGroup, stopper golib.StopChan, graph *collectorGraph) {
	filtered := graph.sortedFilteredNodes()
	if len(filtered) == 0 {
		return
	}
	log.Debugln("Watching filtered collectors:", filtered)

	source.loopCheck(wg, stopper, &filtered, source.FilteredCollectorCheckInterval, func(node *collectorNode) {
		err := node.collector.MetricsChanged()
		if err == MetricsChanged {
			log.Warnln("Metrics of", node, "(filtered) have changed! Restarting metric collection.")
			stopper.Stop()
		} else if err == nil {
			// Reset the update failure counter since there was no error
			node.failedUpdates = 0
		} else {
			log.Warnln("Update of", node, "(filtered) failed:", err)
			if node.updateFailed() {
				graph.modificationLock.Lock()
				filtered = graph.sortedFilteredNodes()
				log.Debugln("Watching filtered collectors:", filtered)
				graph.modificationLock.Unlock()
			}
		}
	})
}

func (source *SampleSource) watchFailedCollectors(wg *sync.WaitGroup, stopper golib.StopChan, graph *collectorGraph) {
	var previousList []*collectorNode
	source.loopCheck(wg, stopper, &graph.failedList, source.FailedCollectorCheckInterval, func(node *collectorNode) {
		// Check if graph.failedList changed in any way
		if len(previousList) != len(graph.failedList) || (len(previousList) > 0 && &(previousList[0]) != &(graph.failedList[0])) {
			log.Debugln("Watching failed collectors:", graph.failedList)
			previousList = graph.failedList
		}

		var err error
		if node.isInitialized() {
			err = node.collector.Update()
		} else {
			_, err = node.init()
		}
		if err == nil {
			log.Warnln("Collector", node, "is not failing anymore. Restarting metric collection.")
			stopper.Stop()
		}
	})
}

func (source *SampleSource) loopCheck(wg *sync.WaitGroup, stopper golib.StopChan, nodes *[]*collectorNode, interval time.Duration, check func(*collectorNode)) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			for _, node := range *nodes {
				check(node)
				if stopper.Stopped() {
					return
				}
			}
			if !stopper.WaitTimeout(interval) {
				return
			}
		}
	}()
}

func (source *SampleSource) PrintMetrics() error {
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

func (source *SampleSource) getGraphForPrinting(fullGraph bool) (*collectorGraph, error) {
	if fullGraph {
		return source.createGraph()
	} else {
		return source.createFilteredGraph()
	}
}

func (source *SampleSource) PrintGraph(file string, fullGraph bool) error {
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

func (source *SampleSource) PrintGraphDot(file string, fullGraph bool) error {
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
