package psutil

import (
	"sync"

	"github.com/bitflow-stream/go-bitflow-collector"
	"github.com/bitflow-stream/go-bitflow/bitflow"
	"github.com/shirou/gopsutil/load"
)

type LoadCollector struct {
	collector.AbstractCollector
	load     load.AvgStat
	loadLock sync.RWMutex
}

func newLoadCollector(root *RootCollector) *LoadCollector {
	return &LoadCollector{
		AbstractCollector: root.Child("load"),
	}
}

func (col *LoadCollector) Metrics() collector.MetricReaderMap {
	return collector.MetricReaderMap{
		"load/1":  col.readLoad1,
		"load/5":  col.readLoad5,
		"load/15": col.readLoad15,
	}
}

func (col *LoadCollector) Update() error {
	loadAvg, err := load.Avg()

	col.loadLock.Lock()
	defer col.loadLock.Unlock()
	if err != nil || loadAvg == nil {
		col.load = load.AvgStat{}
	} else {
		col.load = *loadAvg
	}
	return err
}

func (col *LoadCollector) getLoad() load.AvgStat {
	col.loadLock.Lock()
	defer col.loadLock.Unlock()
	return col.load
}

func (col *LoadCollector) readLoad1() bitflow.Value {
	return bitflow.Value(col.getLoad().Load1)
}

func (col *LoadCollector) readLoad5() bitflow.Value {
	return bitflow.Value(col.getLoad().Load5)
}

func (col *LoadCollector) readLoad15() bitflow.Value {
	return bitflow.Value(col.getLoad().Load15)
}
