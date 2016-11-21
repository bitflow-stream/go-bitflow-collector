package collector

import (
	"fmt"
	"regexp"
	"sort"
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

	RootCollector   CollectorSlice
	CollectInterval time.Duration
	SinkInterval    time.Duration
	ExcludeMetrics  []*regexp.Regexp
	IncludeMetrics  []*regexp.Regexp

	root     *collectorNode
	loopTask *golib.LoopTask
}

func (source *CollectorSource) String() string {
	return fmt.Sprintf("CollectorSource (%v)", source.RootCollector)
}

func (source *CollectorSource) SetSink(sink bitflow.MetricSink) {
	source.OutgoingSink = sink
}

func (source *CollectorSource) Start(wg *sync.WaitGroup) golib.StopChan {
	// TODO integrate golib.StopChan/LoopTask and golib.Stopper
	source.loopTask = golib.NewLoopTask(source.String(), func(stop golib.StopChan) {
		var collectWg sync.WaitGroup
		stopper := source.collect(&collectWg)
		select {
		case <-stopper.Wait():
		case <-stop:
		}
		stopper.Stop()
		collectWg.Wait()
	})
	source.loopTask.StopHook = func() {
		source.CloseSink(wg)
	}
	return source.loopTask.Start(wg)
}

func (source *CollectorSource) Stop() {
	source.loopTask.Stop()
}

func (source *CollectorSource) collect(wg *sync.WaitGroup) *golib.Stopper {
	source.root = &collectorNode{
		collector: source.RootCollector,
	}
	_ = source.root.init() // CollectorSlice.Init() does not fail
	source.root.applyMetricFilters(source.ExcludeMetrics, source.IncludeMetrics)
	source.root.pruneEmptyNodes()
	source.root.initMetrics()

	metrics := source.root.listMetrics()
	sort.Strings(metrics)
	fields := make([]string, len(metrics))
	values := make([]bitflow.Value, len(metrics))
	for i, metric := range metrics {
		fields[i] = metric.name
		metric.index = i
		metric.sample = values
	}
	log.Println("Locally collecting", len(metrics), "metrics through", len(collectors), "collectors")

	stopper := golib.NewStopper()
	source.root.startParallelUpdates(wg, stopper)
	wg.Add(1)
	go source.sinkMetrics(wg, fields, values, source.OutgoingSink, stopper)
	return stopper
}

func (source *CollectorSource) sinkMetrics(wg *sync.WaitGroup, fields []string, values []bitflow.Value, sink bitflow.MetricSink, stopper *golib.Stopper) {
	header := &bitflow.Header{Fields: fields}
	if handler := CollectedSampleHandler; handler != nil {
		handler.HandleHeader(header, CollectorSampleSource)
	}

	defer wg.Done()
	for {
		if err := sink.Header(header); err != nil {
			log.Warnln("Failed to sink header for", len(header.Fields), "metrics:", err)
		} else {
			if stopper.IsStopped() {
				return
			}
			for {
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

func (source *CollectorSource) PrintMetrics() {
	source.root.init()
	all := source.root.listMetricNames()
	source.root.applyMetricFilters(source.ExcludeMetrics, source.IncludeMetrics)
	filtered := source.root.listMetricNames()
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
}
