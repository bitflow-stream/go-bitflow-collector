package collector

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/golib"
)

var MetricsChanged = errors.New("Metrics of this collector have changed")

const (
	FailedCollectorCheckInterval   = 5 * time.Second
	FilteredCollectorCheckInterval = 30 * time.Second
)

type CollectorSource struct {
	CollectInterval time.Duration
	SinkInterval    time.Duration
	ExcludeMetrics  []*regexp.Regexp
	IncludeMetrics  []*regexp.Regexp

	collectors         []Collector
	failedCollectors   []Collector
	filteredCollectors []Collector

	bitflow.AbstractMetricSource
	loopTask *golib.LoopTask
}

func (source *CollectorSource) String() string {
	return "CollectorSource"
}

func (source *CollectorSource) SetSink(sink bitflow.MetricSink) {
	source.OutgoingSink = sink
}

func (source *CollectorSource) Start(wg *sync.WaitGroup) golib.StopChan {
	// TODO integrate golib.StopChan/LoopTask and golib.Stopper
	source.loopTask = golib.NewLoopTask("CollectorSource", func(stop golib.StopChan) {
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
	source.initCollectors()
	metrics := source.FilteredMetrics()
	sort.Strings(metrics)
	header, values, collectors := source.constructSample(metrics)
	log.Println("Locally collecting", len(metrics), "metrics through", len(collectors), "collectors")

	stopper := golib.NewStopper()
	for _, collector := range source.collectors {
		var interval time.Duration
		if _, ok := collectors[collector]; ok {
			interval = source.CollectInterval
		} else {
			interval = FilteredCollectorCheckInterval
		}
		wg.Add(1)
		go source.updateCollector(wg, collector, stopper, interval)
	}
	for _, failed := range source.failedCollectors {
		wg.Add(1)
		go source.watchFailedCollector(wg, failed, stopper)
	}
	wg.Add(1)
	go source.sinkMetrics(wg, header, values, source.OutgoingSink, stopper)
	return stopper
}

func (source *CollectorSource) initCollectors() {
	source.collectors = make([]Collector, 0, len(collectorRegistry))
	source.failedCollectors = make([]Collector, 0, len(collectorRegistry))
	source.filteredCollectors = make([]Collector, 0, len(collectorRegistry))
	for collector, _ := range collectorRegistry {
		if err := source.initCollector(collector); err != nil {
			log.Warnf("%v failed: %v", collector, err)
			source.failedCollectors = append(source.failedCollectors, collector)
			continue
		}
		source.collectors = append(source.collectors, collector)
	}
}

func (source *CollectorSource) initCollector(collector Collector) error {
	if err := collector.Init(); err != nil {
		return err
	}
	if err := collector.Update(); err != nil {
		return err
	}
	return nil
}

func (source *CollectorSource) AllMetrics() []string {
	var all []string
	for _, collector := range source.collectors {
		metrics := collector.SupportedMetrics()
		for _, metric := range metrics {
			all = append(all, metric)
		}
	}
	return all
}

func (source *CollectorSource) FilteredMetrics() (filtered []string) {
	all := source.AllMetrics()
	filtered = make([]string, 0, len(all))
	for _, metric := range all {
		excluded := false
		for _, regex := range source.ExcludeMetrics {
			if excluded = regex.MatchString(metric); excluded {
				break
			}
		}
		if !excluded && len(source.IncludeMetrics) > 0 {
			excluded = true
			for _, regex := range source.IncludeMetrics {
				if excluded = !regex.MatchString(metric); !excluded {
					break
				}
			}
		}
		if !excluded {
			filtered = append(filtered, metric)
		}
	}
	return
}

func (source *CollectorSource) collectorFor(metric string) Collector {
	for _, collector := range source.collectors {
		if collector.SupportsMetric(metric) {
			return collector
		}
	}
	return nil
}

func (source *CollectorSource) constructSample(metrics []string) (*bitflow.Header, []bitflow.Value, map[Collector]bool) {
	set := make(map[Collector]bool)

	fields := make([]string, len(metrics))
	values := make([]bitflow.Value, len(metrics))
	for i, metricName := range metrics {
		collector := source.collectorFor(metricName)
		if collector == nil {
			log.Warnln("No collector found for", metricName)
			continue
		}
		fields[i] = metricName
		metric := Metric{metricName, i, values}

		if err := collector.Collect(&metric); err != nil {
			log.Errorf("Error starting collector for %v: %v", metricName, err)
			continue
		}
		set[collector] = true
	}
	header := &bitflow.Header{Fields: fields}
	if handler := CollectedSampleHandler; handler != nil {
		handler.HandleHeader(header, CollectorSampleSource)
	}
	return header, values, set
}

func (source *CollectorSource) updateCollector(wg *sync.WaitGroup, collector Collector, stopper *golib.Stopper, interval time.Duration) {
	defer wg.Done()
	for {
		err := collector.Update()
		if err == MetricsChanged {
			log.Warnf("Metrics of %v have changed! Restarting metric collection.", collector)
			stopper.Stop()
			return
		} else if err != nil {
			log.Warnln("Update of", collector, "failed:", err)
		}
		if stopper.Stopped(interval) {
			return
		}
	}
}

func (source *CollectorSource) watchFailedCollector(wg *sync.WaitGroup, collector Collector, stopper *golib.Stopper) {
	defer wg.Done()
	for {
		if stopper.Stopped(FailedCollectorCheckInterval) {
			return
		}
		if err := source.initCollector(collector); err == nil {
			log.Warnln("Collector", collector, "is not failing anymore. Restarting metric collection.")
			stopper.Stop()
			return
		}
	}
}

func (source *CollectorSource) sinkMetrics(wg *sync.WaitGroup, header *bitflow.Header, values []bitflow.Value, sink bitflow.MetricSink, stopper *golib.Stopper) {
	defer wg.Done()
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

func (source *CollectorSource) PrintMetrics() {
	source.initCollectors()
	all := source.AllMetrics()
	filtered := source.FilteredMetrics()
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
