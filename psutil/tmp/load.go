package psutil

import (
	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/go-bitflow-collector"
	"github.com/shirou/gopsutil/load"
)

type PsutilLoadCollector struct {
	collector.AbstractCollector
	load *load.AvgStat
}

func (col *PsutilLoadCollector) Init() error {
	col.Reset(col)
	col.Readers = map[string]collector.MetricReader{
		"load/1":  col.readLoad1,
		"load/5":  col.readLoad5,
		"load/15": col.readLoad15,
	}
	return nil
}

func (col *PsutilLoadCollector) Update() (err error) {
	col.load, err = load.Avg()
	if err == nil {
		col.UpdateMetrics()
	}
	return
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
