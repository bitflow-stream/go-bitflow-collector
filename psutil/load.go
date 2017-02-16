package psutil

import (
	"sync"

	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/go-bitflow-collector"
	"github.com/shirou/gopsutil/load"
)

type PsutilLoadCollector struct {
	collector.AbstractCollector
	load     load.AvgStat
	loadLock sync.RWMutex
}

func newLoadCollector(root *PsutilRootCollector) *PsutilLoadCollector {
	return &PsutilLoadCollector{
		AbstractCollector: root.Child("load"),
	}
}

func (col *PsutilLoadCollector) Metrics() collector.MetricReaderMap {
	return collector.MetricReaderMap{
		"load/1":  col.readLoad1,
		"load/5":  col.readLoad5,
		"load/15": col.readLoad15,
	}
}

func (col *PsutilLoadCollector) Update() error {
	loadavg, err := load.Avg()

	col.loadLock.Lock()
	defer col.loadLock.Unlock()
	if err != nil || loadavg == nil {
		col.load = load.AvgStat{}
	} else {
		col.load = *loadavg
	}
	return err
}

func (col *PsutilLoadCollector) getLoad() load.AvgStat {
	col.loadLock.Lock()
	defer col.loadLock.Unlock()
	return col.load
}

func (col *PsutilLoadCollector) readLoad1() bitflow.Value {
	return bitflow.Value(col.getLoad().Load1)
}

func (col *PsutilLoadCollector) readLoad5() bitflow.Value {
	return bitflow.Value(col.getLoad().Load5)
}

func (col *PsutilLoadCollector) readLoad15() bitflow.Value {
	return bitflow.Value(col.getLoad().Load15)
}
