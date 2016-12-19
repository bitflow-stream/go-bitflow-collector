package psutil

import (
	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/go-bitflow-collector"
	"github.com/shirou/gopsutil/load"
)

type PsutilLoadCollector struct {
	collector.AbstractCollector
	load load.AvgStat
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
	if err != nil || loadavg == nil {
		col.load = load.AvgStat{}
	} else {
		col.load = *loadavg
	}
	return err
}

func (col *PsutilLoadCollector) readLoad1() bitflow.Value {
	return bitflow.Value(col.load.Load1)
}

func (col *PsutilLoadCollector) readLoad5() bitflow.Value {
	return bitflow.Value(col.load.Load5)
}

func (col *PsutilLoadCollector) readLoad15() bitflow.Value {
	return bitflow.Value(col.load.Load15)
}
