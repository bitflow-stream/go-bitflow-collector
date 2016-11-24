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
	FilteredCollectorCheckInterval = 30 * time.Second
)

type CollectorSource struct {
	bitflow.AbstractMetricSource

	RootCollectors  []Collector
	CollectInterval time.Duration
	SinkInterval    time.Duration
	ExcludeMetrics  []*regexp.Regexp
	IncludeMetrics  []*regexp.Regexp

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
	graph, err := source.createGraph()
	if err != nil {
		return nil, err
	}

	metrics := graph.getMetrics()
	fields, values := metrics.ConstructSample()
	log.Println("Locally collecting", len(metrics), "metrics through", len(graph.collectors), "collectors")

	stopper := golib.NewStopper()
	source.startParallelUpdates(wg, stopper, graph)
	wg.Add(1)
	go source.sinkMetrics(wg, metrics, fields, values, stopper)
	return stopper, nil
}

func (source *CollectorSource) createGraph() (*collectorGraph, error) {
	graph, err := initCollectorGraph(source.RootCollectors)
	if err != nil {
		return nil, err
	}
	graph.applyMetricFilters(source.ExcludeMetrics, source.IncludeMetrics)
	graph.pruneEmptyNodes()
	return graph, nil
}

func (source *CollectorSource) sinkMetrics(wg *sync.WaitGroup, metrics MetricSlice, fields []string, values []bitflow.Value, stopper *golib.Stopper) {
	header := &bitflow.Header{Fields: fields}
	if handler := CollectedSampleHandler; handler != nil {
		handler.HandleHeader(header, CollectorSampleSource)
	}
	// TODO source.CheckSink() should be called in Start()
	sink := source.OutgoingSink

	defer wg.Done()
	for {
		if err := sink.Header(header); err != nil {
			log.Warnln("Failed to sink header for", len(header.Fields), "metrics:", err)
		} else {
			if stopper.IsStopped() {
				return
			}
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
					// When a sample fails, try sending the header again
					log.Warnln("Failed to sink", len(values), "metrics:", err)
					break
				}
				if stopper.Stopped(source.SinkInterval) {
					return
				}
			}
		}
		if stopper.Stopped(source.SinkInterval) {
			return
		}
	}
}

func (source *CollectorSource) startParallelUpdates(wg *sync.WaitGroup, stopper *golib.Stopper, graph *collectorGraph) {
	plan := graph.createUpdatePlan()
	log.Debugln("Collector update plan:", plan)

	// Update collectors once without forking goroutine
	log.Debugln("Performing initial collector updates...")
	for _, nodes := range plan {
		source.updateCollectors(nodes, stopper)
	}
	log.Debugln("Initial updates complete, now starting background updates")

	for _, nodes := range plan {
		wg.Add(1)
		go source.loopUpdateCollectors(wg, nodes, stopper, source.CollectInterval)
	}
	for node := range graph.failed {
		wg.Add(1)
		go source.loopUpdateCollector(wg, node, stopper, FilteredCollectorCheckInterval)
	}
	wg.Add(1)
	go source.watchFailedCollectors(wg, stopper, graph)
}

func (source *CollectorSource) loopUpdateCollectors(wg *sync.WaitGroup, nodes []*collectorNode, stopper *golib.Stopper, interval time.Duration) {
	defer wg.Done()
	for {
		source.updateCollectors(nodes, stopper)
		if stopper.Stopped(interval) {
			return
		}
	}
}

func (source *CollectorSource) updateCollectors(nodes []*collectorNode, stopper *golib.Stopper) {
	for _, node := range nodes {
		if !source.updateCollector(node, stopper) {
			break
		}
	}
}

func (source *CollectorSource) updateCollector(node *collectorNode, stopper *golib.Stopper) bool {
	err := node.collector.Update()
	if err == MetricsChanged {
		log.Warnln("Metrics of", node, "have changed! Restarting metric collection.")
		stopper.Stop()
		return false
	} else if err != nil {
		log.Warnln("Update of", node, "failed:", err)
		return false
	}
	return true
}

func (source *CollectorSource) loopUpdateCollector(wg *sync.WaitGroup, node *collectorNode, stopper *golib.Stopper, interval time.Duration) {
	defer wg.Done()
	for {
		_ = source.updateCollector(node, stopper)
		if stopper.Stopped(interval) {
			return
		}
	}
}

func (source *CollectorSource) watchFailedCollectors(wg *sync.WaitGroup, stopper *golib.Stopper, graph *collectorGraph) {
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

func (source *CollectorSource) PrintGraph(file string) error {
	graph, err := source.createGraph()
	if err != nil {
		return err
	}

	if !strings.HasSuffix(file, ".png") {
		file += ".png"
	}
	if err := graph.WriteGraph(file); err != nil {
		return fmt.Errorf("Failed to create graph image:", err)
	}
	return nil
}
